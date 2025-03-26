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

package cni_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/spacechunks/explorer/api/platformd/proxy/v1alpha1"
	workloadv1alpha2 "github.com/spacechunks/explorer/api/platformd/workload/v1alpha2"
	"github.com/spacechunks/explorer/cni"
	"github.com/spacechunks/explorer/internal/datapath"
	"github.com/spacechunks/explorer/internal/mock"
	"github.com/spacechunks/explorer/internal/ptr"
	"github.com/stretchr/testify/assert"
	mocky "github.com/stretchr/testify/mock"
)

func TestExecAdd(t *testing.T) {
	tests := []struct {
		name string
		prep func(
			*mock.MockCniHandler,
			*skel.CmdArgs,
			*mock.MockV1alpha1ProxyServiceClient,
			*mock.MockV1alpha2WorkloadServiceClient,
		)
		args *skel.CmdArgs
		conf cni.Conf
		err  error
	}{
		{
			name: "everything works fine",
			conf: cni.Conf{
				NetConf: types.NetConf{
					IPAM: types.IPAM{
						Type: "host-local",
					},
				},
				PlatformdListenSock: "/some/path",
			},
			args: &skel.CmdArgs{
				ContainerID: "abc",
				Args:        "K8S_POD_UID=uuidv7",
				Netns:       "/path/to/netns",
			},
			prep: func(
				h *mock.MockCniHandler,
				args *skel.CmdArgs,
				psc *mock.MockV1alpha1ProxyServiceClient,
				wlc *mock.MockV1alpha2WorkloadServiceClient,
			) {
				var (
					ips = []net.IPNet{
						{
							IP:   net.ParseIP("10.10.0.0"),
							Mask: net.CIDRMask(24, 24),
						},
						{
							IP:   net.ParseIP("10.20.0.0"),
							Mask: net.CIDRMask(24, 24),
						},
					}
					veth = datapath.VethPair{
						HostPeer: datapath.VethPeer{
							Iface: &net.Interface{
								Name: "host",
							},
							Addr: net.ParseIP("10.10.0.0"),
						},
						PodPeer: datapath.VethPeer{
							Iface: &net.Interface{
								Name: "pod",
							},
						},
					}
					port uint32 = 1337
				)

				h.EXPECT().
					AllocIPs("host-local", args.StdinData).
					Return(ips, nil)
				h.EXPECT().
					AllocVethPair(args.Netns, ips[0], ips[1]).
					Return(veth, nil)
				h.EXPECT().
					AttachHostVethBPF(veth).
					Return(nil)
				h.EXPECT().
					AttachCtrVethBPF(veth, args.Netns).
					Return(nil)
				h.EXPECT().
					AddDefaultRoute(veth, args.Netns).
					Return(nil)
				h.EXPECT().
					AddFullMatchRoute(veth).
					Return(nil)

				wlc.EXPECT().
					WorkloadStatus(mocky.Anything, &workloadv1alpha2.WorkloadStatusRequest{
						Id: ptr.Pointer("uuidv7"),
					}).
					Return(&workloadv1alpha2.WorkloadStatusResponse{
						Status: &workloadv1alpha2.WorkloadStatus{
							Port: ptr.Pointer(port),
						},
					}, nil)

				h.EXPECT().
					AddDNATTarget(veth, uint16(1337)).
					Return(nil)

				h.EXPECT().
					AddNetData(datapath.NetData{
						Veth:     veth,
						HostPort: uint16(port),
					}).
					Return(nil)

				psc.EXPECT().
					CreateListeners(mocky.Anything, &v1alpha1.CreateListenersRequest{
						WorkloadID: "uuidv7",
						Ip:         veth.HostPeer.Addr.String(),
					}).
					Return(nil, nil)
			},
		},
		{
			name: "fail when invaild port received",
			conf: cni.Conf{
				NetConf: types.NetConf{
					IPAM: types.IPAM{
						Type: "host-local",
					},
				},
				PlatformdListenSock: "/some/path",
			},
			args: &skel.CmdArgs{
				ContainerID: "abc",
				Args:        "K8S_POD_UID=uuidv7",
				Netns:       "/path/to/netns",
			},
			err: cni.ErrInvalidPort,
			prep: func(
				h *mock.MockCniHandler,
				args *skel.CmdArgs,
				psc *mock.MockV1alpha1ProxyServiceClient,
				wlc *mock.MockV1alpha2WorkloadServiceClient,
			) {
				var (
					ips = []net.IPNet{
						{
							IP:   net.ParseIP("10.10.0.0"),
							Mask: net.CIDRMask(24, 24),
						},
						{
							IP:   net.ParseIP("10.20.0.0"),
							Mask: net.CIDRMask(24, 24),
						},
					}
					veth = datapath.VethPair{}
				)

				h.EXPECT().
					AllocIPs("host-local", args.StdinData).
					Return(ips, nil)
				h.EXPECT().
					AllocVethPair(args.Netns, ips[0], ips[1]).
					Return(veth, nil)
				h.EXPECT().
					AttachHostVethBPF(veth).
					Return(nil)
				h.EXPECT().
					AttachCtrVethBPF(veth, args.Netns).
					Return(nil)
				h.EXPECT().
					AddDefaultRoute(veth, args.Netns).
					Return(nil)
				h.EXPECT().
					AddFullMatchRoute(veth).
					Return(nil)

				wlc.EXPECT().
					WorkloadStatus(mocky.Anything, &workloadv1alpha2.WorkloadStatusRequest{
						Id: ptr.Pointer("uuidv7"),
					}).
					Return(&workloadv1alpha2.WorkloadStatusResponse{
						Status: &workloadv1alpha2.WorkloadStatus{},
					}, nil)

				h.EXPECT().
					DeallocIPs("host-local", args.StdinData).
					Return(nil)
			},
		},
		{
			name: "dealloc ips on error",
			conf: cni.Conf{
				NetConf: types.NetConf{
					IPAM: types.IPAM{
						Type: "host-local",
					},
				},
				PlatformdListenSock: "/some/path",
			},
			args: &skel.CmdArgs{
				Args: "K8S_POD_UID=uuidv7",
			},
			err: fmt.Errorf("configure veth pair: some error"),
			prep: func(
				h *mock.MockCniHandler,
				args *skel.CmdArgs,
				_ *mock.MockV1alpha1ProxyServiceClient,
				_ *mock.MockV1alpha2WorkloadServiceClient,
			) {
				h.EXPECT().
					AllocIPs("host-local", args.StdinData).
					Return([]net.IPNet{{}, {}}, nil)
				h.EXPECT().
					AllocVethPair(args.Netns, mocky.Anything, mocky.Anything).
					Return(datapath.VethPair{}, fmt.Errorf("some error"))
				h.EXPECT().
					DeallocIPs("host-local", args.StdinData).
					Return(nil)
			},
		},
		{
			name: "fail if ipam config is not set",
			conf: cni.Conf{
				PlatformdListenSock: "/some/path",
			},
			args: &skel.CmdArgs{
				Args: "K8S_POD_UID=uuidv7",
			},
			err: cni.ErrIPAMConfigNotSet,
			prep: func(
				_ *mock.MockCniHandler,
				_ *skel.CmdArgs,
				_ *mock.MockV1alpha1ProxyServiceClient,
				_ *mock.MockV1alpha2WorkloadServiceClient,
			) {
			},
		},
		{
			name: "fail if platformd listen sock is not set",
			conf: cni.Conf{},
			args: &skel.CmdArgs{
				Args: "K8S_POD_UID=uuidv7",
			},
			err: cni.ErrPlatformdListenSockNotSet,
			prep: func(
				_ *mock.MockCniHandler,
				_ *skel.CmdArgs,
				_ *mock.MockV1alpha1ProxyServiceClient,
				_ *mock.MockV1alpha2WorkloadServiceClient,
			) {
			},
		},
		{
			name: "fail K8S_POD_UID in CNI_ARGS is not set",
			conf: cni.Conf{
				NetConf: types.NetConf{
					IPAM: types.IPAM{
						Type: "host-local",
					},
				},
				PlatformdListenSock: "/some/path",
			},
			args: &skel.CmdArgs{
				Args: "K8S_POD_NAMESPACE=abc",
			},
			err: cni.ErrPodUIDMissing,
			prep: func(
				_ *mock.MockCniHandler,
				_ *skel.CmdArgs,
				_ *mock.MockV1alpha1ProxyServiceClient,
				_ *mock.MockV1alpha2WorkloadServiceClient,
			) {
			},
		},
		{
			name: "fail if in CNI_ARGS is malformed",
			conf: cni.Conf{
				NetConf: types.NetConf{
					IPAM: types.IPAM{
						Type: "host-local",
					},
				},
				PlatformdListenSock: "/some/path",
			},
			args: &skel.CmdArgs{
				Args: "K8S_POD_NAMESPACE=abc,K8S_POD_NAME=",
			},
			err: fmt.Errorf("CNI_ARGS parse error: invalid CNI_ARGS pair \"K8S_POD_NAMESPACE=abc,K8S_POD_NAME=\""),
			prep: func(
				_ *mock.MockCniHandler,
				_ *skel.CmdArgs,
				_ *mock.MockV1alpha1ProxyServiceClient,
				_ *mock.MockV1alpha2WorkloadServiceClient,
			) {
			},
		},
		{
			name: "fail if we have less than two ip addresses",
			conf: cni.Conf{
				NetConf: types.NetConf{
					IPAM: types.IPAM{
						Type: "host-local",
					},
				},
				PlatformdListenSock: "/some/path",
			},
			args: &skel.CmdArgs{
				Args: "K8S_POD_UID=uuidv7",
			},
			err: cni.ErrInsufficientAddresses,
			prep: func(
				h *mock.MockCniHandler,
				args *skel.CmdArgs,
				_ *mock.MockV1alpha1ProxyServiceClient,
				_ *mock.MockV1alpha2WorkloadServiceClient,
			) {
				h.EXPECT().
					AllocIPs("host-local", args.StdinData).
					Return([]net.IPNet{{}}, nil)
				h.EXPECT().
					DeallocIPs("host-local", args.StdinData).
					Return(nil)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				h   = mock.NewMockCniHandler(t)
				psc = mock.NewMockV1alpha1ProxyServiceClient(t)
				wlc = mock.NewMockV1alpha2WorkloadServiceClient(t)
				c   = cni.NewCNI(h)
			)
			tt.prep(h, tt.args, psc, wlc)
			err := c.ExecAdd(tt.args, tt.conf, psc, wlc)
			if err != nil && tt.err != nil {
				assert.ErrorAs(t, err, &tt.err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
