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
	"testing"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/spacechunks/platform/internal/cni"
	"github.com/spacechunks/platform/internal/datapath"
	"github.com/spacechunks/platform/internal/mock"
	"github.com/stretchr/testify/assert"
)

func TestExecAdd(t *testing.T) {
	tests := []struct {
		name string
		prep func(*mock.MockCniHandler, *skel.CmdArgs)
		args *skel.CmdArgs
		err  string
	}{
		{
			name: "everything works fine",
			args: &skel.CmdArgs{
				ContainerID: "abc",
				Netns:       "/path/to/netns",
				StdinData:   []byte(`{"hostIface":"eth0","ipam":{"type":"host-local"}}`),
			},
			prep: func(h *mock.MockCniHandler, args *skel.CmdArgs) {
				h.EXPECT().
					AttachDNATBPF("eth0").
					Return(nil)
				h.EXPECT().
					AllocIPs("host-local", args.StdinData).
					Return(nil, nil)
				h.EXPECT().
					AllocVethPair(args.Netns, nil, nil).
					Return(datapath.VethPair{}, nil)
				h.EXPECT().
					AttachHostVethBPF("hostVeth").
					Return(nil)
				h.EXPECT().
					ConfigureSNAT(nil, "").
					Return(nil)
				h.EXPECT().
					AddDefaultRoute(args.Netns, ""). // TODO fix
					Return(nil)
			},
		},
		{
			name: "dealloc ips on error",
			args: &skel.CmdArgs{
				StdinData: []byte(`{"hostIface":"eth0","ipam":{"type":"host-local"}}`),
			},
			err: "alloc ips: some error",
			prep: func(h *mock.MockCniHandler, args *skel.CmdArgs) {
				h.EXPECT().
					AttachDNATBPF("eth0").
					Return(nil)
				h.EXPECT().
					AllocIPs("host-local", args.StdinData).
					Return(nil, errors.New("some error"))
				h.EXPECT().
					DeallocIPs("host-local", args.StdinData).
					Return(nil)
			},
		},
		{
			name: "fail if hostIface is not set",
			args: &skel.CmdArgs{
				StdinData: []byte(`{"ipam":{"type":"host-local"}}`),
			},
			err: cni.ErrHostIfaceNotFound.Error(),
			prep: func(h *mock.MockCniHandler, args *skel.CmdArgs) {
			},
		},
		{
			name: "fail if ipam config is not set",
			args: &skel.CmdArgs{
				StdinData: []byte(`{}`),
			},
			err: cni.ErrIPAMConfigNotFound.Error(),
			prep: func(h *mock.MockCniHandler, args *skel.CmdArgs) {
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				h = mock.NewMockCniHandler(t)
				c = cni.NewCNI(h)
			)
			tt.prep(h, tt.args)
			err := c.ExecAdd(tt.args)
			if err != nil && tt.err != "" {
				assert.EqualError(t, err, tt.err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
