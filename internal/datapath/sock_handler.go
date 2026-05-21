package datapath

import (
	"fmt"
	"strings"

	"github.com/containernetworking/plugins/pkg/ns"
)

type SockHandler interface {
	BlockNewConnections(criCgroupsPath string) error
	DestroySocks(netnsPath string) error
}

func NewLinuxSockHandler(objs *Objects) *LinuxSockHandler {
	return &LinuxSockHandler{
		bpf: objs,
	}
}

type LinuxSockHandler struct {
	bpf *Objects
}

func (h LinuxSockHandler) BlockNewConnections(criCgroupsPath string) error {
	// the cgroup path returned by the container info is not a path
	// that is found in the filesystem. cgroupData returns a path to
	// the cgroup on the current filesystem.
	cgroupPath, err := cgroupData(criCgroupsPath)
	if err != nil {
		return fmt.Errorf("cgroup data: %w", err)
	}

	if err := h.bpf.BlockIP4Connections(cgroupPath); err != nil {
		return fmt.Errorf("block ip4: %w", err)
	}

	if err := h.bpf.BlockIP6Connections(cgroupPath); err != nil {
		return fmt.Errorf("block ip6: %w", err)
	}

	return nil
}

func (h LinuxSockHandler) DestroySocks(netnsPath string) error {
	// it is very important that we run the socket destruction
	// inside the network namespace of the container, so the
	// ebpf program knows what sockets to kill.
	if err := ns.WithNetNSPath(netnsPath, func(netNS ns.NetNS) error {
		if err := h.bpf.DestroyTCPSocks(); err != nil {
			return fmt.Errorf("destroy tcp socks: %w", err)
		}

		if err := h.bpf.DestroyUDPSocks(); err != nil {
			return fmt.Errorf("destroy tcp socks: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("in netns: %w", err)
	}
	return nil
}

func cgroupData(cgroupsPath string) (string, error) {
	// cgroupsPath looks like this: system.slice:crio:<container-id>
	parts := strings.Split(cgroupsPath, ":")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid cgroups path: %s", cgroupsPath)
	}

	path := fmt.Sprintf("/sys/fs/cgroup/%s/%s-%s.scope/container", parts[0], parts[1], parts[2])
	return path, nil
}
