#include "types.h"
#include "bpf/bpf_helpers.h"
#include "bpf/bpf_endian.h"
#include "vmlinux.h"

extern int bpf_sock_destroy(struct sock_common *sk) __ksym;

SEC("cgroup/connect4")
int block_connect4(struct bpf_sock_addr *ctx)
{
    // block all connections for ipv4
    return 0;
}

SEC("cgroup/connect6")
int block_connect6(struct bpf_sock_addr *ctx)
{
    // block all connections for ipv6
    return 0;
}

SEC("iter/tcp")
int destroy_tcp(struct bpf_iter__tcp *ctx)
{
    struct seq_file *seq = ctx->meta->seq;
    struct sock_common* sk = ctx->sk_common;
    if (sk == NULL)
        return 0;

    struct net* netns = sk->skc_net.net;
    if (netns == NULL)
        return 0;

    struct task_struct *task = bpf_get_current_task_btf();

    __u32 sockns = netns->ns.inum;
    __u32 tskns = task->nsproxy->net_ns->ns.inum;

    /*
     * only destroy sockets in the netns of the container to checkpoint.
     */
    if (tskns != sockns)
        return 0;

    /* as we don't want to kill the mc server, we just skip all listening sockets */
    if (sk->skc_state == TCP_LISTEN) {
        return 0;
    }

    /* FIXME: log when there is a failure */
    bpf_sock_destroy(sk);
    return 0;
}

SEC("iter/udp")
int destroy_udp(struct bpf_iter__udp *ctx)
{
    struct seq_file *seq = ctx->meta->seq;
    struct udp_sock* usk = ctx->udp_sk;
    if (usk == NULL)
        return 0;

    struct inet_sock* inet = &usk->inet;
    struct sock_common* sk = &inet->sk.__sk_common;
    struct task_struct *task = bpf_get_current_task_btf();

    __u32 sockns = sk->skc_net.net->ns.inum;
    __u32 tskns = task->nsproxy->net_ns->ns.inum;

    /*
     * only destroy sockets in the netns of the container to checkpoint.
     */
    if (tskns != sockns)
        return 0;

    /*
     * skip udp sockets that do not have a destination port set.
     * those are most likely udp servers, we don't want to kill
     * them as of now. those could be voice chat servers for example.
     */
    if (bpf_ntohs(sk->skc_dport) == 0)
        return 0;

    /* FIXME: log when there is a failure */
    bpf_sock_destroy(sk);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
