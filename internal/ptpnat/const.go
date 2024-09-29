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

package ptpnat

import "net"

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang-18 -strip llvm-strip-18 snat bpf/snat.c -- -I bpf/include
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang-18 -strip llvm-strip-18 dnat bpf/dnat.c -- -I bpf/include

const (
	VethMTU = 1400

	supportedCNIVersion = "1.0.0"
	pinPath             = "/sys/fs/bpf"
)

var (
	// PodVethCIDR is the IPv4 CIDR configured for the pod-side veth
	PodVethCIDR = mustParseCIDR("10.0.0.1/24")

	// HostVethMAC is the mac address configured for the host-side veth
	HostVethMAC = mustParseMAC("7e:90:c4:ed:df:d0")
)

func mustParseMAC(s string) net.HardwareAddr {
	mac, err := net.ParseMAC(s)
	if err != nil {
		panic(err)
	}
	return mac
}

func mustParseCIDR(cidr string) *net.IPNet {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	// for some reason the host part is lost
	// in ipNet. 10.0.0.1/24 -> 10.0.0.0/24
	return &net.IPNet{
		IP:   ip,
		Mask: ipNet.Mask,
	}
}
