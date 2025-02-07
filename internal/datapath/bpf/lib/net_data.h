struct veth_peer {
    __u32 if_index;
    __be32 if_addr;
    __u8 mac_addr[ETH_ALEN];
};

/* this struct holds all networking information related to a workload. */
struct net_data {
    struct veth_peer pod_peer;
    struct veth_peer host_peer;
    __u16 host_port;
};

/*
 * you can retrieve net_data using one of the following values as keys
 * - host port
 * - pod peer address
 */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, __u32);
    __type(value, struct net_data);
    __uint(max_entries, 256); /* TODO: determine sane value */
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} net_data_map SEC(".maps");

