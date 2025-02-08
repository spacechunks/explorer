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

package cni

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	proxyv1alpha1 "github.com/spacechunks/platform/api/platformd/proxy/v1alpha1"
	workloadv1alpha1 "github.com/spacechunks/platform/api/platformd/workload/v1alpha1"
	"github.com/spacechunks/platform/internal/datapath"
)

var (
	ErrPlatformdListenSockNotSet = errors.New("platformd listen socket not set")
	ErrIPAMConfigNotSet          = errors.New("ipam config not set")
	ErrPodUIDMissing             = errors.New("K8S_POD_UID in CNI_ARGS missing")
	ErrInsufficientAddresses     = errors.New("ipam: need 2 ip addresses")
)

type Conf struct {
	types.NetConf
	PlatformdListenSock string `json:"platformdListenSock"`
}

type CNI struct {
	handler Handler
}

func NewCNI(h Handler) *CNI {
	return &CNI{
		handler: h,
	}
}

// ExecAdd sets up the veth pair for a container.
// internally the following happens:
// * first allocated ip address for host side veth using cni ipam plugin.
// * then create veth pair and move one peer into the containers netns.
// * configure ip address on container iface and bring it up.
// * configure ip address on host iface and bring it up.
// * attach snat bpf program to host-side veth peer (tc ingress)
func (c *CNI) ExecAdd(
	args *skel.CmdArgs,
	conf Conf,
	proxyClient proxyv1alpha1.ProxyServiceClient,
	wlClient workloadv1alpha1.WorkloadServiceClient,
) (err error) {
	ctx := context.Background()

	cniArgs, err := parseArgs(args.Args)
	if err != nil {
		return fmt.Errorf("CNI_ARGS parse error: %v", err)
	}

	// workload service sets the pod uid to the workloads ID
	wlID, ok := cniArgs["K8S_POD_UID"]
	if !ok {
		return ErrPodUIDMissing
	}

	if conf.PlatformdListenSock == "" {
		return ErrPlatformdListenSockNotSet
	}

	if conf.IPAM == (types.IPAM{}) {
		return ErrIPAMConfigNotSet
	}

	defer func() {
		if err != nil {
			if err := c.handler.DeallocIPs(conf.IPAM.Type, args.StdinData); err != nil {
				log.Printf("could not deallocate ips after CNI ADD failure: %v\n", err)
			}
		}
	}()

	ips, err := c.handler.AllocIPs(conf.IPAM.Type, args.StdinData)
	if err != nil {
		return fmt.Errorf("alloc ips: %w", err)
	}

	if len(ips) < 2 {
		return ErrInsufficientAddresses
	}

	veth, err := c.handler.AllocVethPair(args.Netns, ips[0] /* host */, ips[1] /* pod */)
	if err != nil {
		return fmt.Errorf("configure veth pair: %w", err)
	}

	if err := c.handler.AttachHostVethBPF(veth); err != nil {
		return fmt.Errorf("attach host peer: %w", err)
	}

	if err := c.handler.AttachCtrVethBPF(veth, args.Netns); err != nil {
		return fmt.Errorf("attach ctr peer: %w", err)
	}

	if err := c.handler.AddDefaultRoute(veth, args.Netns); err != nil {
		return fmt.Errorf("add default route: %w", err)
	}

	if err := c.handler.AddFullMatchRoute(veth); err != nil {
		return fmt.Errorf("add full match route: %w", err)
	}

	resp, err := wlClient.WorkloadStatus(ctx, &workloadv1alpha1.WorkloadStatusRequest{
		Id: wlID,
	})
	if err != nil {
		return fmt.Errorf("get workload status: %w", err)
	}

	port := uint16(resp.Status.Port)

	if err := c.handler.AddDNATTarget(veth, port); err != nil {
		return fmt.Errorf("add dnat target: %w", err)
	}

	if err := c.handler.AddNetData(datapath.NetData{
		Veth:     veth,
		HostPort: port,
	}); err != nil {
		return fmt.Errorf("add net data: %w", err)
	}

	if _, err := proxyClient.CreateListeners(ctx, &proxyv1alpha1.CreateListenersRequest{
		WorkloadID: wlID,
		Ip:         veth.HostPeer.Addr.IP.String(),
	}); err != nil {
		return fmt.Errorf("create proxy listeners: %w", err)
	}

	result := &current.Result{
		CNIVersion: supportedCNIVersion,
		Interfaces: []*current.Interface{
			{
				Name:    veth.PodPeer.Iface.Name,
				Sandbox: args.Netns,
			},
		},
	}

	if err := result.PrintTo(os.Stdout); err != nil {
		return fmt.Errorf("print result: %w", err)
	}

	return nil
}

func (c *CNI) ExecDel(args *skel.CmdArgs) error {
	log.Println("del")
	// TODO: remove resources in this order:
	//  - remove host route
	//  - remove proxy listeners
	//  - remove veth pairs
	//  - remove ebpf map entries
	//  - dealloc ips
	return nil
}

func parseArgs(args string) (map[string]string, error) {
	var (
		ret   = make(map[string]string)
		pairs = strings.Split(args, ";")
	)
	for _, pair := range pairs {
		kv := strings.Split(pair, "=")
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return nil, fmt.Errorf("invalid CNI_ARGS pair %q", pair)
		}
		ret[kv[0]] = kv[1]
	}
	return ret, nil
}
