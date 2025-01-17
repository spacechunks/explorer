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

package workload

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// have this in as a *_internal_test, so we can access
// internal fields for testing purposes.

func TestPortAllocation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		prep func() *PortAllocator
	}{
		{
			name: "allocate multiple ports successfully",
			prep: func() *PortAllocator {
				return NewPortAllocator(1000, 2000)
			},
		},
		{
			name: "port allocation failed",
			prep: func() *PortAllocator {
				return &PortAllocator{
					portMin: 0,
					portMax: 2,
					allocated: map[int]bool{
						0: true,
						1: true,
						2: true,
					},
				}
			},
			err: ErrMaxPortTriesReached,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allocator := tt.prep()
			_, err := allocator.Allocate()
			if tt.err != nil {
				require.Equal(t, tt.err, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
