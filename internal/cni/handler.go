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
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/spacechunks/platform/internal/datapath"

	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ipam"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// just for reference: _ctr_ is short for _container_

type Handler interface {
	CreateAndConfigureVethPair(netNS string, ips []*current.IPConfig) (string, string, error)

	// AttachHostVethBPF installs all BPF programs intended for the host-side veth peer
	AttachHostVethBPF(ifaceName string) error

	AttachCtrVethBPF(ifaceName string, netNS string) error
	AllocIPs(plugin string, stdinData []byte) ([]*current.IPConfig, error)
	DeallocIPs(plugin string, stdinData []byte) error
	AttachDNATBPF(ifaceName string) error
	ConfigureSNAT(ifaceName string) error
	AddDefaultRoute(nsPath string, ip netip.Addr) error

	// AddFullMatchRoute will create a rule in the root ns, which routes packets
	// with the fully matching ip address (/32 CIDR) to the given interface.
	AddFullMatchRoute(ifname string, ip netip.Addr) error
}

type cniHandler struct {
	bpf *datapath.Objects
}

func NewHandler() (Handler, error) {
	objs, err := datapath.LoadBPF()
	if err != nil {
		return nil, err
	}

	return &cniHandler{
		bpf: objs,
	}, nil
}

func (h *cniHandler) AttachHostVethBPF(ifaceName string) error {
	iface, err := datapath.IfaceByName(ifaceName)
	if err != nil {
		return fmt.Errorf("get iface: %w", err)
	}
	//
	//if err := h.bpf.AttachAndPinSNAT(iface); err != nil {
	//	return fmt.Errorf("snat: %w", err)
	//}

	if err := h.bpf.AttachAndPinARP(iface); err != nil {
		return fmt.Errorf("arp: %w", err)
	}

	if err := h.bpf.AttachTProxyHostEgress(iface); err != nil {
		return fmt.Errorf("tproxy host egress: %w", err)
	}

	return nil
}

func (h *cniHandler) AttachCtrVethBPF(ifaceName string, netNS string) error {
	ctrNS, err := ns.GetNS(netNS)
	if err != nil {
		return fmt.Errorf("get netns: %w", err)
	}

	if err := ctrNS.Do(func(_ ns.NetNS) error {
		iface, err := datapath.IfaceByName(ifaceName)
		if err != nil {
			return fmt.Errorf("get iface: %w", err)
		}

		if err := h.bpf.AttachTProxyCtrEgress(iface); err != nil {
			return fmt.Errorf("tproxy ctr egress: %w", err)
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (h *cniHandler) CreateAndConfigureVethPair(netNS string, ips []*current.IPConfig) (string, string, error) {
	hostVethName, err := randHexStr()
	if err != nil {
		return "", "", fmt.Errorf("could not generate host-side veth name: %w", err)
	}

	podVethName, err := randHexStr()
	if err != nil {
		return "", "", fmt.Errorf("could not generate pod-side veth name: %w", err)
	}

	ctrNS, err := createAndMoveVethPair(hostVethName, podVethName, netNS)
	if err != nil {
		return "", "", fmt.Errorf("setup veth pair: %w", err)
	}

	defer ctrNS.Close()

	hostPeerAddr := ips[0].Address
	podPeerAddr := ips[1].Address

	if err := configureCTRPeer(ctrNS, podPeerAddr, podVethName); err != nil {
		return "", "", fmt.Errorf("setup ctr side veth: %w", err)
	}

	if err := configureHostPeer(hostPeerAddr, hostVethName); err != nil {
		return "", "", fmt.Errorf("setup host side veth: %w", err)
	}

	hostPeer, err := net.InterfaceByName(hostVethName)
	if err != nil {
		return "", "", fmt.Errorf("get host peer iface: %w", err)
	}

	var ctrPeer *net.Interface
	if err := ctrNS.Do(func(ns.NetNS) error {
		ctrPeer, err = net.InterfaceByName(podVethName)
		if err != nil {
			return fmt.Errorf("get ctr peer iface: %w", err)
		}
		return nil
	}); err != nil {
		return "", "", err
	}

	if err := h.bpf.AddVethPairEntry(
		uint32(hostPeer.Index),
		uint32(ctrPeer.Index),
		netip.MustParseAddr(hostPeerAddr.IP.String()),
	); err != nil {
		return "", "", fmt.Errorf("put veth pairs: %w", err)
	}

	return hostVethName, podVethName, nil
}

func (h *cniHandler) AllocIPs(plugin string, stdinData []byte) ([]*current.IPConfig, error) {
	ipamRes, err := ipam.ExecAdd(plugin, stdinData)
	if err != nil {
		return nil, fmt.Errorf("ipam: %v", err)
	}

	// convert ipam result into the current versions result type
	result, err := current.NewResultFromResult(ipamRes)
	if err != nil {
		return nil, fmt.Errorf("convert ipam result: %v", err)
	}

	if len(result.IPs) == 0 {
		return nil, errors.New("ipam plugin returned missing IPs")
	}

	return result.IPs, nil
}

func (h *cniHandler) DeallocIPs(plugin string, stdinData []byte) error {
	return ipam.ExecDel(plugin, stdinData)
}

func (h *cniHandler) AttachDNATBPF(ifaceName string) error {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return fmt.Errorf("get iface: %w", err)
	}

	if err := h.bpf.AttachAndPinDNAT(datapath.Iface{Name: ifaceName, Index: iface.Index}); err != nil {
		return fmt.Errorf("attach dnat bpf: %w", err)
	}

	return nil
}

func (h *cniHandler) ConfigureSNAT(ifaceName string) error {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return fmt.Errorf("get iface: %w", err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return fmt.Errorf("get iface addrs: %w", err)
	}

	if len(addrs) == 0 {
		return errors.New("no addresses configured")
	}

	prefix, err := netip.ParsePrefix(addrs[0].String())
	if err != nil {
		return fmt.Errorf("parse addr: %w", err)
	}

	if err := h.bpf.AddSNATTarget(0, prefix.Addr(), uint8(iface.Index)); err != nil {
		return fmt.Errorf("add snat target: %w", err)
	}
	return nil
}

func (h *cniHandler) AddDefaultRoute(nsPath string, addr netip.Addr) error {
	if err := ns.WithNetNSPath(nsPath, func(_ ns.NetNS) error {
		// for default gateway we can leave destination empty.
		// we also do not need to specify the device, the kernel
		// will figure this out for us.
		if err := netlink.RouteAdd(&netlink.Route{
			Gw:     net.ParseIP(addr.String()),
			Family: unix.AF_INET,
			Scope:  netlink.SCOPE_LINK,
		}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return fmt.Errorf("add default route: %w", err)
	}
	return nil
}

func (h *cniHandler) AddFullMatchRoute(ifname string, ip netip.Addr) error {
	iface, err := net.InterfaceByName(ifname)
	if err != nil {
		return fmt.Errorf("get iface: %w", err)
	}
	s := &net.IPNet{
		IP:   net.ParseIP(ip.String()),
		Mask: net.IPv4Mask(255, 255, 255, 255), // /32
	}
	if err := netlink.RouteAdd(&netlink.Route{
		LinkIndex: iface.Index,
		Scope:     netlink.SCOPE_LINK,
		Dst:       s,
		Family:    unix.AF_INET,
	}); err != nil {
		return err
	}
	return nil
}

func configureCTRPeer(ctrNS ns.NetNS, ip net.IPNet, ifaceName string) error {
	if err := ctrNS.Do(func(ns.NetNS) error {
		return configureIface(ifaceName, &ip, nil)
	}); err != nil {
		return fmt.Errorf("ctr ns: %w", err)
	}
	return nil
}

func configureHostPeer(ip net.IPNet, ifaceName string) error {
	if err := configureIface(ifaceName, &ip, &HostVethMAC); err != nil {
		return fmt.Errorf("configure iface (%s): %w", ip.String(), err)
	}
	return nil
}

// configureIface sets the given ip and optionally also the mac address.
// if mac is nil the hardware address will not be set.
func configureIface(ifaceName string, ipNet *net.IPNet, mac *net.HardwareAddr) error {
	l, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("lookup link: %w", err)
	}

	if err := netlink.AddrAdd(l, &netlink.Addr{IPNet: ipNet}); err != nil {
		return fmt.Errorf("add addr: %w", err)
	}

	// When using systemd `MacAddressPolicy` needs to be set to `none`.
	// Otherwise, there appears to be a race condition where our configured
	// mac address will not be picked up. This is because since version 242,
	// systemd will set a persistent mac address on virtual interfaces.
	// see
	// * https://lore.kernel.org/netdev/CAHXsExy+zm+twpC9Qrs9myBre+5s_ApGzOYU45Pt=sw-FyOn1w@mail.gmail.com/
	// * https://github.com/Mellanox/mlxsw/wiki/Persistent-Configuration#required-changes-to-macaddresspolicy
	if mac != nil {
		if err := netlink.LinkSetHardwareAddr(l, *mac); err != nil {
			return fmt.Errorf("set hardware addr: %w", err)
		}
	}

	if err := netlink.LinkSetUp(l); err != nil {
		return fmt.Errorf("link up again: %w", err)
	}

	return nil
}

func createAndMoveVethPair(hostVethName, podVethName, netNS string) (ns.NetNS, error) {
	vethpair := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: podVethName,
			MTU:  VethMTU,
		},
		PeerName: hostVethName,
	}

	if err := netlink.LinkAdd(vethpair); err != nil {
		return nil, fmt.Errorf("add veth: %w", err)
	}

	ctrNS, err := ns.GetNS(netNS)
	if err != nil {
		return nil, fmt.Errorf("get netns fd: %w", err)
	}

	if err := netlink.LinkSetNsFd(vethpair, int(ctrNS.Fd())); err != nil {
		return nil, fmt.Errorf("move pod veth to ns %d: %w", ctrNS, err)
	}

	return ctrNS, nil
}

func randHexStr() (string, error) {
	bytes := make([]byte, 16) // are enough to achieve a negligible collision chance
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// return first 15 chars
	return fmt.Sprintf("%x", bytes)[:15], nil
}
