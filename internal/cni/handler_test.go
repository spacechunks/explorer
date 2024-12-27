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
	"errors"
	"net"
	"testing"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/spacechunks/platform/internal/cni"
	"github.com/spacechunks/platform/test"
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

var stdinData = []byte(`
{
  "cniVersion": "1.0.0",
  "name":"test",
  "ipam":{
    "type": "host-local", 
    "ranges":[
      [{"subnet": "10.0.10.0/24"}]
    ]
  }
}
`)

// TestSetup tests that ip address and mac address could be allocated
// and configured on the veth-pairs.
func TestIfaceConfig(t *testing.T) {
	var (
		handle, name = test.CreateNetns(t)
		ctrID        = "ABC"
		nsPath       = "/var/run/netns/" + name
	)

	h, err := cni.NewHandler()
	require.NoError(t, err)

	defer func() {
		h.DeallocIPs("host-local", stdinData)
		handle.Close()
		netns.DeleteNamed(name)
	}()

	// host-local cni plugin requires container id
	test.SetCNIEnvVars(ctrID, "ignored", nsPath)

	ips, err := h.AllocIPs("host-local", stdinData)
	require.NoError(t, err)

	veth, err := h.AllocVethPair(nsPath, ips[0].Address, ips[1].Address)
	require.NoError(t, err)

	var (
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
		test.RequireAddrConfigured(t, podVethName, cni.PodVethCIDR.String())
		return nil
	})
	require.NoError(t, err)

	test.RequireAddrConfigured(t, hostVethName, ips[0].Address.String())
	require.Equal(t, cni.HostVethMAC.String(), hostVeth.Attrs().HardwareAddr.String())
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
		{
			name: "no addresses configured",
			prep: func(t *testing.T, veth netlink.Link) {},
			err:  errors.New("no addresses configured"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, link := test.AddRandVethPair(t)

			_, err := cni.NewHandler()
			require.NoError(t, err)

			tt.prep(t, link)
			defer netlink.LinkDel(link)

			if tt.err != nil {
				//require.EqualError(t, h.ConfigureSNAT(iface.Name), tt.err.Error())
				return
			}

			//require.NoError(t, h.ConfigureSNAT(iface.Name))
		})
	}
}
