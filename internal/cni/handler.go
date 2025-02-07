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
	"fmt"
	"net"

	"github.com/spacechunks/platform/internal/datapath"

	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ipam"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// just for reference: _ctr_ is short for _container_

type Handler interface {
	AllocVethPair(netNS string, hostAddr, podAddr net.IPNet) (datapath.VethPair, error)

	// AttachHostVethBPF installs all BPF programs intended for the host-side veth peer
	AttachHostVethBPF(veth datapath.VethPair) error

	AttachCtrVethBPF(veth datapath.VethPair, netNS string) error
	AllocIPs(plugin string, stdinData []byte) ([]net.IPNet, error)
	DeallocIPs(plugin string, stdinData []byte) error
	AddDefaultRoute(veth datapath.VethPair, nsPath string) error

	// AddFullMatchRoute will create a rule in the root ns, which routes packets
	// with the fully matching ip address (/32 CIDR) to the given interface.
	AddFullMatchRoute(veth datapath.VethPair) error

	// AddDNATTarget maps the passed port to the veth pairs pod peer.
	AddDNATTarget(veth datapath.VethPair, port uint16) error

	AddNetData(data datapath.NetData) error
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

func (h *cniHandler) AttachHostVethBPF(veth datapath.VethPair) error {
	if err := h.bpf.AttachAndPinSNAT(veth.HostPeer.Iface); err != nil {
		return fmt.Errorf("snat: %w", err)
	}

	if err := h.bpf.AttachAndPinARP(veth.HostPeer.Iface); err != nil {
		return fmt.Errorf("arp: %w", err)
	}

	if err := h.bpf.AttachTProxyHostEgress(veth.HostPeer.Iface); err != nil {
		return fmt.Errorf("tproxy host egress: %w", err)
	}

	return nil
}

func (h *cniHandler) AttachCtrVethBPF(veth datapath.VethPair, netNS string) error {
	ctrNS, err := ns.GetNS(netNS)
	if err != nil {
		return fmt.Errorf("get netns: %w", err)
	}

	if err := ctrNS.Do(func(_ ns.NetNS) error {
		if err := h.bpf.AttachTProxyCtrEgress(veth.PodPeer.Iface); err != nil {
			return fmt.Errorf("tproxy ctr egress: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (h *cniHandler) AllocVethPair(netNS string, hostAddr, podAddr net.IPNet) (datapath.VethPair, error) {
	hostVethName, err := randHexStr()
	if err != nil {
		return datapath.VethPair{}, fmt.Errorf("could not generate host-side veth name: %w", err)
	}

	podVethName, err := randHexStr()
	if err != nil {
		return datapath.VethPair{}, fmt.Errorf("could not generate pod-side veth name: %w", err)
	}

	ctrNS, err := createAndMoveVethPair(hostVethName, podVethName, netNS)
	if err != nil {
		return datapath.VethPair{}, fmt.Errorf("setup veth pair: %w", err)
	}

	defer ctrNS.Close()

	if err := configureCTRPeer(ctrNS, podAddr, podVethName); err != nil {
		return datapath.VethPair{}, fmt.Errorf("setup ctr side veth: %w", err)
	}

	if err := configureHostPeer(hostAddr, hostVethName); err != nil {
		return datapath.VethPair{}, fmt.Errorf("setup host side veth: %w", err)
	}

	hostPeer, err := net.InterfaceByName(hostVethName)
	if err != nil {
		return datapath.VethPair{}, fmt.Errorf("get host peer iface: %w", err)
	}

	var podPeer *net.Interface
	if err := ctrNS.Do(func(ns.NetNS) error {
		podPeer, err = net.InterfaceByName(podVethName)
		if err != nil {
			return fmt.Errorf("get ctr peer iface: %w", err)
		}
		return nil
	}); err != nil {
		return datapath.VethPair{}, err
	}

	if err := h.bpf.AddVethPairEntry(
		uint32(hostPeer.Index),
		uint32(podPeer.Index),
		hostAddr.IP,
	); err != nil {
		return datapath.VethPair{}, fmt.Errorf("put veth pairs: %w", err)
	}

	return datapath.VethPair{
		HostPeer: datapath.VethPeer{
			Iface: hostPeer,
			Addr:  hostAddr,
		},
		PodPeer: datapath.VethPeer{
			Iface: podPeer,
			Addr:  podAddr,
		},
	}, nil
}

func (h *cniHandler) AllocIPs(plugin string, stdinData []byte) ([]net.IPNet, error) {
	ipamRes, err := ipam.ExecAdd(plugin, stdinData)
	if err != nil {
		return nil, fmt.Errorf("ipam: %v", err)
	}

	// convert ipam result into the current versions result type
	result, err := current.NewResultFromResult(ipamRes)
	if err != nil {
		return nil, fmt.Errorf("convert ipam result: %v", err)
	}

	addrs := make([]net.IPNet, 0, len(result.IPs))
	for _, i := range result.IPs {
		addrs = append(addrs, i.Address)
	}

	return addrs, nil
}

func (h *cniHandler) DeallocIPs(plugin string, stdinData []byte) error {
	return ipam.ExecDel(plugin, stdinData)
}

func (h *cniHandler) AddDefaultRoute(veth datapath.VethPair, nsPath string) error {
	if err := ns.WithNetNSPath(nsPath, func(_ ns.NetNS) error {
		// for default gateway we can leave destination empty.
		// we also do not need to specify the device, the kernel
		// will figure this out for us.
		if err := netlink.RouteAdd(&netlink.Route{
			Gw:     veth.PodPeer.Addr.IP,
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

func (h *cniHandler) AddFullMatchRoute(veth datapath.VethPair) error {
	s := &net.IPNet{
		IP:   veth.PodPeer.Addr.IP,
		Mask: net.IPv4Mask(255, 255, 255, 255), // /32
	}
	if err := netlink.RouteAdd(&netlink.Route{
		LinkIndex: veth.HostPeer.Iface.Index,
		Scope:     netlink.SCOPE_LINK,
		Dst:       s,
		Family:    unix.AF_INET,
	}); err != nil {
		return err
	}
	return nil
}

func (h *cniHandler) AddDNATTarget(veth datapath.VethPair, port uint16) error {
	return h.bpf.AddDNATTarget(
		port,
		veth.PodPeer.Addr.IP,
		// use host peer index, because bpf_redirect_peer
		// redirects packets to ifaces peer device. in this
		// case the pod peer.
		uint8(veth.HostPeer.Iface.Index),
		veth.PodPeer.Iface.HardwareAddr,
	)
}

func (h *cniHandler) AddNetData(data datapath.NetData) error {
	return h.bpf.AddNetData(data)
}

func configureCTRPeer(ctrNS ns.NetNS, ip net.IPNet, ifaceName string) error {
	if err := ctrNS.Do(func(ns.NetNS) error {
		return configureIface(ifaceName, ip, nil)
	}); err != nil {
		return fmt.Errorf("ctr ns: %w", err)
	}
	return nil
}

func configureHostPeer(ip net.IPNet, ifaceName string) error {
	if err := configureIface(ifaceName, ip, &HostVethMAC); err != nil {
		return fmt.Errorf("configure iface (%s): %w", ip.String(), err)
	}
	return nil
}

// configureIface sets the given ip and optionally also the mac address.
// if mac is nil the hardware address will not be set.
func configureIface(ifaceName string, ip net.IPNet, mac *net.HardwareAddr) error {
	l, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("lookup link: %w", err)
	}

	if err := netlink.AddrAdd(l, &netlink.Addr{IPNet: &ip}); err != nil {
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
