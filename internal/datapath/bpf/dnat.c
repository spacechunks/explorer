/*
Explorer Platform, a platform for hosting and discovering Minecraft servers.
Copyright (C) 2024 Yannic Rieger <oss@76k.io>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

#include "types.h"
#include "bpf/bpf_helpers.h"
#include "bpf/bpf_endian.h"
#include "linux/if_ether.h"
#include "vmlinux.h"
#include "net_helpers.h"

#define TCP_DPORT_OFF (ETH_HLEN + sizeof(struct iphdr) + offsetof(struct tcphdr, dest))
#define IP_DST_OFF (ETH_HLEN + offsetof(struct iphdr, daddr))

struct dnat_target {
    __u32 ip_addr; /* host byte order */
    __u8 iface_idx;
    __u8 mac_addr[ETH_ALEN];
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, __u16); /* host byte order */
    __type(value, struct dnat_target);
    __uint(max_entries, 256); /* TODO: determine sane value */
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} ptp_dnat_targets SEC(".maps");

SEC("tc")
int dnat(struct __sk_buff *ctx)
{
    void *data = (void *)(long)ctx->data;
    void *data_end = (void *)(long)ctx->data_end;

    if (ctx->protocol != bpf_htons(ETH_P_IP))
        return TC_ACT_OK;

    __be16 nport;
    bpf_skb_load_bytes(ctx, TCP_DPORT_OFF, &nport, sizeof(__be16));

    __u16 hport = bpf_ntohs(nport);
    struct dnat_target *tgt = bpf_map_lookup_elem(&ptp_dnat_targets, &hport);

    if (tgt == NULL) {
        /* prevent SSH connections from spamming */
        if (hport != 22) bpf_printk("no dnat target for port %d", hport);
        return TC_ACT_OK;
    }

    struct ethhdr *ethh;
    if (parse_ethhdr(&data, data_end, &ethh))
        return TC_ACT_OK;

    __builtin_memcpy(ethh->h_dest, tgt->mac_addr, ETH_ALEN);

    __be32 dst = bpf_htonl(tgt->ip_addr);
    __be32 prev_dst;

    bpf_skb_load_bytes(ctx, IP_DST_OFF, &prev_dst, IP_ADDR_LEN);
    bpf_skb_store_bytes(ctx, IP_DST_OFF, &dst, sizeof(dst), 0);
    bpf_l3_csum_replace(ctx, IP_CSUM_OFF, prev_dst, dst, sizeof(dst));
    bpf_l4_csum_replace(ctx, TCP_CSUM_OFF, prev_dst, dst,  BPF_F_PSEUDO_HDR | sizeof(dst));

    return bpf_redirect_peer(tgt->iface_idx, 0);
}

char _license[] SEC("license") = "GPL";
