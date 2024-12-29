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

package functional

import (
	"net"
	"testing"

	"github.com/cilium/ebpf/link"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/spacechunks/platform/internal/cni"
	"github.com/spacechunks/platform/internal/datapath"
	"github.com/spacechunks/platform/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// we use github.com/vishvananda/netns library and
// github.com/containernetworking/plugins/pkg/ns
// because
// * github.com/vishvananda/netns
//   provides us with the ability to create/destroy named network namespaces.
//   the other one does not provide this feature.
// * github.com/containernetworking/plugins/pkg/ns
//   provides us with the ability to execute functions in the context of
//   a given network namespace.

// TestAllocVethPair tests that ip address and mac address could be allocated
// and configured on the veth-pairs.
func TestAllocVethPair(t *testing.T) {
	h, err := cni.NewHandler()
	require.NoError(t, err)

	var (
		nsPath, veth = setup(t, h)
		podVethName  = veth.PodPeer.Iface.Name
		hostVethName = veth.HostPeer.Iface.Name
	)

	podVeth := test.GetLinkByNS(t, podVethName, nsPath)

	hostVeth, err := netlink.LinkByName(hostVethName)
	require.NoError(t, err)

	require.NotNil(t, podVeth, "pod veth not found")
	require.NotNil(t, hostVeth, "host veth not found")
	require.Equal(t, cni.VethMTU, podVeth.Attrs().MTU)

	err = ns.WithNetNSPath(nsPath, func(netNS ns.NetNS) error {
		test.RequireAddrConfigured(t, podVethName, veth.PodPeer.Addr.String())
		return nil
	})
	require.NoError(t, err)

	test.RequireAddrConfigured(t, hostVethName, veth.HostPeer.Addr.String())
	require.Equal(t, cni.HostVethMAC.String(), hostVeth.Attrs().HardwareAddr.String())
}

// TestAttachHostVethBPF checks that all needed bpf programs are installed on
// the host peer interface
func TestAttachHostVethBPF(t *testing.T) {
	h, err := cni.NewHandler()
	require.NoError(t, err)

	_, veth := setup(t, h)

	pins := []string{
		datapath.ProgPinPath + "/snat_" + veth.HostPeer.Iface.Name,
		datapath.ProgPinPath + "/arp_" + veth.HostPeer.Iface.Name,
		datapath.ProgPinPath + "/host_peer_egress_" + veth.HostPeer.Iface.Name,
	}

	require.NoError(t, h.AttachHostVethBPF(veth))

	for _, p := range pins {
		l, err := link.LoadPinnedLink(p, nil)
		require.NoError(t, err)

		info, err := l.Info()
		require.NoError(t, err)

		assert.Equal(t, uint32(veth.HostPeer.Iface.Index), info.TCX().Ifindex)
	}
}

func TestConfigureSNAT(t *testing.T) {
	tests := []struct {
		name string
		prep func(*testing.T, netlink.Link)
		err  error
	}{
		{
			name: "works",
			prep: func(t *testing.T, veth netlink.Link) {
				require.NoError(t, netlink.AddrAdd(veth, &netlink.Addr{
					IPNet: &net.IPNet{
						IP:   net.ParseIP("10.0.0.1"),
						Mask: []byte{255, 255, 255, 0},
					},
				}))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iface, link := test.AddRandVethPair(t)

			h, err := cni.NewHandler()
			require.NoError(t, err)

			tt.prep(t, link)
			defer netlink.LinkDel(link)

			addrs, err := iface.Addrs()
			require.NoError(t, err)

			ip, _, err := net.ParseCIDR(addrs[0].String())
			require.NoError(t, err)

			require.NoError(t, h.ConfigureSNAT(ip, uint8(iface.Index)))
		})
	}
}

func setup(t *testing.T, h cni.Handler) (string, datapath.VethPair) {
	var (
		handle, name = test.CreateNetns(t)
		ctrID        = "ABC"
		nsPath       = "/var/run/netns/" + name
		stdinData    = []byte(
			`{"cniVersion": "1.0.0","name":"test","ipam":{"type": "host-local","ranges":[[{"subnet": "10.1.1.0/24"}],[{"subnet": "10.2.2.0/24"}]]}}`) //nolint:lll
	)

	t.Cleanup(func() {
		h.DeallocIPs("host-local", stdinData)
		handle.Close()
		netns.DeleteNamed(name)
	})

	// host-local cni plugin requires container id
	test.SetCNIEnvVars(ctrID, "ignored", nsPath)

	ips, err := h.AllocIPs("host-local", stdinData)
	require.NoError(t, err)

	veth, err := h.AllocVethPair(nsPath, ips[0].Address, ips[1].Address)
	require.NoError(t, err)

	return nsPath, veth
}
