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

package test

import (
	"net"
	"os"
	"runtime"
	"testing"

	"github.com/containernetworking/plugins/pkg/ns"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// SetCNIEnvVars sets environment variables needed in order
// to make cni IPAM plugin work.
func SetCNIEnvVars(containerID, ifname, cniNetNS string) {
	_ = os.Setenv("CNI_CONTAINERID", containerID)
	_ = os.Setenv("CNI_IFNAME", ifname)
	_ = os.Setenv("CNI_NETNS", cniNetNS)
}

func CreateNetns(t *testing.T) (netns.NsHandle, string) {
	// lock the OS Thread, so we don't accidentally switch namespaces
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// generate random netns name to avoid collisions
	// when running multiple tests at once.
	name := RandHexStr(t)
	origin, err := netns.Get()
	if err != nil {
		t.Fatalf("create netns: %v", err)
	}

	handle, err := netns.NewNamed(name)
	if err != nil {
		t.Fatalf("create netns: %v", err)
	}
	if err := netns.Set(origin); err != nil {
		t.Fatalf("set netns: %v", err)
	}
	return handle, name
}

func GetLinkByNS(t *testing.T, name string, nsPath string) netlink.Link {
	var l netlink.Link
	err := ns.WithNetNSPath(nsPath, func(_ ns.NetNS) error {
		tmp, err := netlink.LinkByName(name)
		if err != nil {
			t.Fatalf("get link by name (%s): %v", name, err)
		}
		l = tmp
		return nil
	})
	require.NoError(t, err)
	return l
}

// AddRandVethPair adds a veth pair with a random name.
// This is mostly used for tests where a dummy network
// interface is needed.
func AddRandVethPair(t *testing.T) (*net.Interface, netlink.Link) {
	var (
		ifaceName = RandHexStr(t)
		vethpair  = &netlink.Veth{
			LinkAttrs: netlink.LinkAttrs{
				Name: ifaceName,
			},
			PeerName: ifaceName + "-p",
		}
	)
	require.NoError(t, netlink.LinkAdd(vethpair))
	iface, err := net.InterfaceByName(ifaceName)
	require.NoError(t, err)

	return iface, vethpair
}

func RequireAddrConfigured(t *testing.T, ifaceName, expectedAddr string) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		t.Fatalf("get iface by name (%s): %v", ifaceName, err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		t.Fatalf("get addrs of iface (%s): %v", ifaceName, err)
	}
	for _, addr := range addrs {
		if addr.String() == expectedAddr {
			return
		}
	}
	t.Fatalf("expected %s to be configured", expectedAddr)
}
