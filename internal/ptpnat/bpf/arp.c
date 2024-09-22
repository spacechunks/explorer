
#include "types.h"
#include "bpf/bpf_helpers.h"
#include "bpf/bpf_endian.h"
#include "vmlinux.h"

#define ARP_OP_OFF  20
#define ARP_SHA_OFF 22
#define ARP_SIP_OFF 28
#define ARP_THA_OFF 32
#define ARP_TIP_OFF 38

int arp(struct __sk_buff *ctx)
{
    void *data = (void *)(long)ctx->data;
    void *data_end = (void *)(long)ctx->data_end;

    if (ctx->protocol != bpf_htons(ETH_P_ARP))
        return TC_ACT_OK;

    struct ethhdr *ethh;
    if (parse_ethhdr(&data, data_end, &ethh))
        return TC_ACT_OK;

    /* copy source, because we will override it later when swapping source and destination */
    __u8 eth_src_tmp[ETH_ALEN];
    __builtin_memcpy(eth_src_tmp, ethh->h_source, ETH_ALEN);

    __builtin_memcpy(ethh->h_source, ethh->h_dest, ETH_ALEN);
    __builtin_memcpy(ethh->h_dest, eth_src_tmp, ETH_ALEN);

    __be16 arp_op;
    bpf_skb_load_bytes(ctx, ARP_OP_OFF, &arp_op, sizeof(arp_op));

    if (bpf_ntohs(arp_op) != ARPOP_REQUEST)
        return TC_ACT_OK;

    __be32 sip, tip;
    bpf_skb_load_bytes(ctx, ARP_TIP_OFF, &tip, sizeof(tip));
    bpf_skb_load_bytes(ctx, ARP_SIP_OFF, &sip, sizeof(sip));

    arp_op = bpf_htons(ARPOP_REPLY);

    bpf_skb_store_bytes(ctx, ARP_OP_OFF, &arp_op, sizeof(arp_op), 0);
    bpf_skb_store_bytes(ctx, ARP_SHA_OFF, &eth_src, ETH_ALEN, 0);
    bpf_skb_store_bytes(ctx, ARP_THA_OFF, &eth_dst, ETH_ALEN, 0);
    bpf_skb_store_bytes(ctx, ARP_SIP_OFF, &tip, sizeof(tip), 0);
    bpf_skb_store_bytes(ctx, ARP_TIP_OFF, &sip, sizeof(sip), 0);

	return bpf_redirect(ctx->ifindex, 0);
}
