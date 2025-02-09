//go:build functests

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
	"net"
	"testing"

	"github.com/cilium/ebpf"
	"github.com/stretchr/testify/require"
)

// TestNetData ensures that every required combination of keys
// is added and removed correctly from the map.
func TestNetData(t *testing.T) {
	objs, err := LoadBPF()
	require.NoError(t, err)

	netDataFixture := func(cidrStr string, hostPort uint16) NetData {
		ip, cidr, _ := net.ParseCIDR(cidrStr)
		return NetData{
			Veth: VethPair{
				HostPeer: VethPeer{
					Iface: &net.Interface{
						Index:        1,
						HardwareAddr: []byte{1, 2, 3, 4, 5, 6},
					},
					Addr: net.IPNet{
						IP:   ip,
						Mask: cidr.Mask,
					},
				},
				PodPeer: VethPeer{
					Iface: &net.Interface{
						Index:        2,
						HardwareAddr: []byte{1, 2, 3, 4, 5, 6},
					},
					Addr: net.IPNet{
						IP:   ip,
						Mask: cidr.Mask,
					},
				},
			},
			HostPort: hostPort,
		}
	}

	loadMaps := func() []*ebpf.Map {
		var dnatMaps dnatMaps
		err = loadDnatObjects(&dnatMaps, &ebpf.CollectionOptions{
			Maps: ebpf.MapOptions{
				PinPath: mapPinPath,
			},
		})
		require.NoError(t, err)

		var snatMaps snatMaps
		err = loadSnatObjects(&snatMaps, &ebpf.CollectionOptions{
			Maps: ebpf.MapOptions{
				PinPath: mapPinPath,
			},
		})
		require.NoError(t, err)

		return []*ebpf.Map{
			dnatMaps.NetDataMap,
			snatMaps.NetDataMap,
		}
	}

	tests := []struct {
		name  string
		data  NetData
		check func(*testing.T, NetData)
	}{
		{
			name: "add netdata",
			// use different values here, so we don't have run into conflicts,
			// because the underlying map will not be cleared
			data: netDataFixture("198.51.100.1/32", 1337),
			check: func(t *testing.T, fixture NetData) {
				err = objs.AddNetData(fixture)
				require.NoError(t, err)

				expectedMapValue := netDataToMapValue(fixture)

				for _, m := range loadMaps() {
					var value dnatNetData
					err = m.Lookup(uint32(fixture.HostPort), &value)
					require.NoError(t, err)
					require.Equal(t, expectedMapValue, value)

					err = m.Lookup(expectedMapValue.PodPeer.IfAddr, &value)
					require.NoError(t, err)
					require.Equal(t, expectedMapValue, value)
				}
			},
		},
		{
			name: "delete netdata",
			data: netDataFixture("198.51.100.2/32", 420),
			check: func(t *testing.T, fixture NetData) {
				err = objs.AddNetData(fixture)
				require.NoError(t, err)

				err = objs.DelNetData(fixture)
				require.NoError(t, err)

				for _, m := range loadMaps() {
					var value dnatNetData
					err = m.Lookup(uint32(fixture.HostPort), &value)
					require.ErrorIs(t, err, ebpf.ErrKeyNotExist)

					err = m.Lookup(binary.BigEndian.Uint32(fixture.Veth.PodPeer.Addr.IP.To4()), &value)
					require.ErrorIs(t, err, ebpf.ErrKeyNotExist)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, tt.data)
		})
	}
}

// TestAddSNATTarget ensures that a target is added with the correct values to the map.
// this is important, because the ip address needs to be correctly converted to an uint32.
// testing DelSNATTarget is not needed, since there is no special functionality, that
// requires a dedicated test.
func TestAddSNATTarget(t *testing.T) {
	objs, err := LoadBPF()
	require.NoError(t, err)

	var (
		ip       = net.ParseIP("10.0.0.1")
		ifaceIdx = uint8(3)
		expected = snatPtpSnatEntry{
			IpAddr:   uint32(167772161), // 10.0.0.1 in big endian decimal
			IfaceIdx: ifaceIdx,
		}
	)

	require.NoError(t, objs.AddSNATTarget(0, ip, ifaceIdx))

	var maps snatMaps
	err = loadSnatObjects(&maps, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: mapPinPath,
		},
	})
	require.NoError(t, err)

	var actual snatPtpSnatEntry
	err = maps.PtpSnatConfig.Lookup(uint8(0), &actual)
	require.NoError(t, err)

	require.Equal(t, expected, actual)
}

// TestAddDNATTarget ensures that a target is added with the correct values to the map.
// this is important, because the ip address needs to be correctly converted to an uint32.
// testing DelDNATTarget is not needed, since there is no special functionality, that
// requires a dedicated test.
func TestAddDNATTarget(t *testing.T) {
	objs, err := LoadBPF()
	require.NoError(t, err)

	var (
		ip       = net.ParseIP("10.0.0.1")
		ifaceIdx = uint8(3)
		hwAddr   = net.HardwareAddr{
			0x7e, 0x90, 0xc4, 0xed, 0xdf, 0xd0,
		}
		expected = dnatDnatTarget{
			IpAddr:   uint32(167772161), // 10.0.0.1 in big endian decimal
			IfaceIdx: ifaceIdx,
			MacAddr: [6]uint8{
				0x7e, 0x90, 0xc4, 0xed, 0xdf, 0xd0,
			},
		}
	)

	require.NoError(t, objs.AddDNATTarget(0, ip, ifaceIdx, hwAddr))

	var maps dnatMaps
	err = loadDnatObjects(&maps, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: mapPinPath,
		},
	})
	require.NoError(t, err)

	var actual dnatDnatTarget
	err = maps.PtpDnatTargets.Lookup(uint16(0), &actual)
	require.NoError(t, err)

	require.Equal(t, expected, actual)
}
