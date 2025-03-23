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

package datapath

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang-18 -strip llvm-strip-18 snat ./bpf/snat.c -- -I ./bpf -I ./bpf/include
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang-18 -strip llvm-strip-18 dnat ./bpf/dnat.c -- -I ./bpf -I ./bpf/include
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang-18 -strip llvm-strip-18 arp ./bpf/arp.c -- -I ./bpf/ -I ./bpf/include
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang-18 -strip llvm-strip-18 tproxy ./bpf/tproxy.c -- -I ./bpf -I ./bpf/include

const (
	ProgPinPath = "/sys/fs/bpf/progs"
	mapPinPath  = "/sys/fs/bpf/maps"
)

type Objects struct {
	snatObjs   snatObjects
	dnatObjs   dnatObjects
	arpObjs    arpObjects
	tproxyObjs tproxyObjects
}

func LoadBPF() (*Objects, error) {
	if err := os.MkdirAll(ProgPinPath, 0777); err != nil {
		return nil, fmt.Errorf("create prog dir: %w", err)
	}

	if err := os.MkdirAll(mapPinPath, 0777); err != nil {
		return nil, fmt.Errorf("create map dir: %w", err)
	}

	var snatObjs snatObjects
	if err := loadSnatObjects(&snatObjs, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: mapPinPath,
		},
	}); err != nil {
		return nil, fmt.Errorf("load snat objs: %w", err)
	}

	var dnatObjs dnatObjects
	if err := loadDnatObjects(&dnatObjs, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: mapPinPath,
		},
	}); err != nil {
		return nil, fmt.Errorf("load dnat objs: %w", err)
	}

	var arpObjs arpObjects
	if err := loadArpObjects(&arpObjs, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: mapPinPath,
		},
	}); err != nil {
		return nil, fmt.Errorf("load arp objs: %w", err)
	}

	var tproxyObjs tproxyObjects
	if err := loadTproxyObjects(&tproxyObjs, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: mapPinPath,
		},
	}); err != nil {
		return nil, fmt.Errorf("load tproxy objs: %w", err)
	}

	return &Objects{
		snatObjs:   snatObjs,
		dnatObjs:   dnatObjs,
		arpObjs:    arpObjs,
		tproxyObjs: tproxyObjs,
	}, nil
}

func (o *Objects) AttachAndPinSNAT(iface *net.Interface) error {
	l, err := link.AttachTCX(link.TCXOptions{
		Interface: iface.Index,
		Program:   o.snatObjs.Snat,
		Attach:    ebpf.AttachTCXIngress,
	})
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	// pin because cni is short-lived
	if err := l.Pin(fmt.Sprintf("%s/snat_%s", ProgPinPath, iface.Name)); err != nil {
		return fmt.Errorf("pin link: %w", err)
	}

	return nil
}

func (o *Objects) AttachAndPinDNAT(iface *net.Interface) error {
	l, err := link.AttachTCX(link.TCXOptions{
		Interface: iface.Index,
		Program:   o.dnatObjs.Dnat,
		Attach:    ebpf.AttachTCXIngress,
	})
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	// pin, so in case platformd crashes the program is still running
	if err := l.Pin(fmt.Sprintf("%s/dnat_%s", ProgPinPath, iface.Name)); err != nil {
		if errors.Is(err, os.ErrExist) {
			// TODO: update prog
			return nil
		}
		return fmt.Errorf("pin link: %w", err)
	}

	return nil
}

func (o *Objects) AttachAndPinARP(iface *net.Interface) error {
	l, err := link.AttachTCX(link.TCXOptions{
		Interface: iface.Index,
		Program:   o.arpObjs.Arp,
		Attach:    ebpf.AttachTCXIngress,
	})
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	// pin because cni is short-lived
	if err := l.Pin(fmt.Sprintf("%s/arp_%s", ProgPinPath, iface.Name)); err != nil {
		return fmt.Errorf("pin link: %w", err)
	}

	return nil
}

func (o *Objects) AttachAndPinGetsockopt(cgroupPath string) error {
	l, err := link.AttachCgroup(link.CgroupOptions{
		Path:    cgroupPath,
		Attach:  ebpf.AttachCGroupGetsockopt,
		Program: o.tproxyObjs.Getsockopt,
	})
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}
	if err := l.Pin(fmt.Sprintf("%s/cgroup_getsockopt", ProgPinPath)); err != nil {
		if errors.Is(err, os.ErrExist) {
			// TODO: update prog
			return nil
		}
		return fmt.Errorf("pin: %w", err)
	}

	return nil
}

func (o *Objects) AttachTProxyHostEgress(hostPeer *net.Interface) error {
	l, err := link.AttachTCX(link.TCXOptions{
		Interface: hostPeer.Index,
		Program:   o.tproxyObjs.HostPeerEgress,
		Attach:    ebpf.AttachTCXEgress,
	})
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if err := l.Pin(fmt.Sprintf("%s/host_peer_egress_%s", ProgPinPath, hostPeer.Name)); err != nil {
		return fmt.Errorf("pin: %w", err)
	}

	return nil
}

func (o *Objects) AttachTProxyCtrEgress(ctrPeer *net.Interface) error {
	l, err := link.AttachTCX(link.TCXOptions{
		Interface: ctrPeer.Index,
		Program:   o.tproxyObjs.CtrPeerEgress,
		Attach:    ebpf.AttachTCXEgress,
	})
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if err := l.Pin(fmt.Sprintf("%s/ctr_peer_egress_%s", ProgPinPath, ctrPeer.Name)); err != nil {
		return fmt.Errorf("pin: %w", err)
	}

	return nil
}

func (o *Objects) AddNetData(data NetData) error {
	var (
		value       = netDataToMapValue(data)
		podPeerAddr = value.PodPeer.IfAddr
	)

	// snat and dnat use the same underlying map, so it is sufficient
	// to only use dnatObjs.

	if err := o.dnatObjs.NetDataMap.Put(uint32(data.HostPort), value); err != nil {
		return fmt.Errorf("add dnat by port: %w", err)
	}

	if err := o.dnatObjs.NetDataMap.Put(podPeerAddr, value); err != nil {
		return fmt.Errorf("add dnat by addr: %w", err)
	}

	return nil
}

func (o *Objects) GetNetData(port uint16) (NetData, error) {
	var value dnatNetData
	if err := o.dnatObjs.NetDataMap.Lookup(uint32(port), &value); err != nil {
		return NetData{}, fmt.Errorf("get by port: %w", err)
	}

	return NetData{
		Veth: VethPair{
			HostPeer: VethPeer{
				Iface: &net.Interface{
					Index:        int(value.HostPeer.IfIndex),
					HardwareAddr: value.HostPeer.MacAddr[:],
				},
				Addr: intToIP(value.HostPeer.IfAddr),
			},
			PodPeer: VethPeer{
				Iface: &net.Interface{
					Index:        int(value.PodPeer.IfIndex),
					HardwareAddr: value.PodPeer.MacAddr[:],
				},
				Addr: intToIP(value.PodPeer.IfAddr),
			},
		},
		HostPort: value.HostPort,
	}, nil
}

func (o *Objects) DelNetData(data NetData) error {
	podPeerAddr := ipToInt(data.Veth.PodPeer.Addr)

	// see comment in AddNetData

	if err := o.dnatObjs.NetDataMap.Delete(uint32(data.HostPort)); err != nil {
		return fmt.Errorf("remove by port: %w", err)
	}

	if err := o.dnatObjs.NetDataMap.Delete(podPeerAddr); err != nil {
		return fmt.Errorf("remove by addr: %w", err)
	}

	return nil
}

func (o *Objects) AddDNATTarget(key uint16, ip net.IP, ifaceIdx uint8, mac net.HardwareAddr) error {
	if err := o.dnatObjs.PtpDnatTargets.Put(key, dnatDnatTarget{
		IpAddr:   ipToInt(ip),
		IfaceIdx: ifaceIdx,
		MacAddr:  [6]byte(mac),
	}); err != nil {
		return err
	}

	return nil
}

func (o *Objects) DelDNATTarget(port uint16) error {
	return o.dnatObjs.PtpDnatTargets.Delete(port)
}

func (o *Objects) AddSNATTarget(key uint8, ip net.IP, ifaceIdx uint8) error {
	if err := o.snatObjs.PtpSnatConfig.Put(key, snatPtpSnatEntry{
		IpAddr:   ipToInt(ip),
		IfaceIdx: ifaceIdx,
	}); err != nil {
		return err
	}

	return nil
}

func (o *Objects) DelSNATTarget(key uint8) error {
	return o.snatObjs.PtpSnatConfig.Delete(key)
}

func (o *Objects) AddVethPairEntry(veth VethPair) error {
	mapValue := tproxyVethPair{
		HostIfIndex: uint32(veth.HostPeer.Iface.Index),
		HostIfAddr:  ipToInt(veth.HostPeer.Addr),
	}

	if err := o.tproxyObjs.VethPairMap.Put(uint32(veth.HostPeer.Iface.Index), mapValue); err != nil {
		return fmt.Errorf("host: %w", err)
	}

	if err := o.tproxyObjs.VethPairMap.Put(uint32(veth.PodPeer.Iface.Index), mapValue); err != nil {
		return fmt.Errorf("pod: %w", err)
	}

	return nil
}

func (o *Objects) DelVethPairEntry(veth VethPair) error {
	if err := o.tproxyObjs.VethPairMap.Delete(uint32(veth.HostPeer.Iface.Index)); err != nil {
		return fmt.Errorf("delete using host peer index: %w", err)
	}

	if err := o.tproxyObjs.VethPairMap.Delete(uint32(veth.PodPeer.Iface.Index)); err != nil {
		return fmt.Errorf("delete using pod peer index: %w", err)
	}

	return nil
}

func netDataToMapValue(data NetData) dnatNetData {
	return dnatNetData{
		PodPeer: struct {
			IfIndex uint32
			IfAddr  uint32
			MacAddr [6]uint8
			_       [2]byte
		}{
			IfIndex: uint32(data.Veth.PodPeer.Iface.Index),
			IfAddr:  ipToInt(data.Veth.PodPeer.Addr),
			MacAddr: [6]byte(data.Veth.PodPeer.Iface.HardwareAddr[:]),
		},
		HostPeer: struct {
			IfIndex uint32
			IfAddr  uint32
			MacAddr [6]uint8
			_       [2]byte
		}{
			IfIndex: uint32(data.Veth.HostPeer.Iface.Index),
			IfAddr:  ipToInt(data.Veth.HostPeer.Addr),
			MacAddr: [6]byte(data.Veth.HostPeer.Iface.HardwareAddr[:]),
		},
		HostPort: data.HostPort,
	}
}

func intToIP(val uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, val)
	return ip
}

func ipToInt(ip net.IP) uint32 {
	return binary.BigEndian.Uint32(ip.To4())
}
