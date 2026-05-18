#include "types.h"
#include "bpf/bpf_helpers.h"
#include "bpf/bpf_tracing.h"
#include "vmlinux.h"

char LICENSE[] SEC("license") = "GPL";

SEC("lsm/socket_create")
int BPF_PROG(restrict_create, int family, int type, int protocol, int kern, int ret)
{
    __u64 dest = bpf_get_current_cgroup_id();
    bpf_printk("lsm: %d", dest);
    return 0;
}