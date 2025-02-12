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

package datapath_test

import (
	"fmt"
	"testing"

	"github.com/spacechunks/explorer/internal/datapath"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/spacechunks/explorer/test"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
)

func TestAttachBPFProgs(t *testing.T) {
	objects, err := datapath.LoadBPF()
	require.NoError(t, err)

	tests := []struct {
		name               string
		pinPrefix          string
		expectedAttachType uint32
		attach             func(string, int) error
	}{
		{
			name:      "attach dnat",
			pinPrefix: "dnat_",
			// BPF_TCX_INGRESS
			// see https://github.com/cilium/ebpf/blob/625b0a910e1ba666e483e75b149880ce3b54dc85/internal/sys/types.go#L229
			expectedAttachType: 46,
			attach:             objects.AttachAndPinDNAT,
		},
		{
			name:               "attach snat",
			pinPrefix:          "snat_",
			expectedAttachType: 46,
			attach:             objects.AttachAndPinSNAT,
		},
		{
			name:               "attach arp",
			pinPrefix:          "arp_",
			expectedAttachType: 46,
			attach:             objects.AttachAndPinARP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iface, veth := test.AddRandVethPair(t)
			defer netlink.LinkDel(veth)

			require.NoError(t, tt.attach(iface.Name, iface.Index))

			pin := fmt.Sprintf("%s/ptp_%s%s", "/sys/fs/bpf", tt.pinPrefix, iface.Name)

			l, err := link.LoadPinnedLink(pin, &ebpf.LoadPinOptions{})
			require.NoError(t, err)

			defer l.Unpin()

			info, err := l.Info()
			require.NoError(t, err)

			require.Equal(t, tt.expectedAttachType, uint32(info.TCX().AttachType))
		})
	}
}
