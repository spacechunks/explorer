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

package workload_test

import (
	"testing"

	"github.com/spacechunks/explorer/platformd/workload"
	"github.com/stretchr/testify/require"
)

func TestStatusStoreUpdate(t *testing.T) {
	tests := []struct {
		name     string
		status   workload.Status
		expected workload.Status
	}{
		{
			name: "update port only",
			status: workload.Status{
				Port: 1337,
			},
			expected: workload.Status{
				Port: 1337,
			},
		},
		{
			name: "update state only",
			status: workload.Status{
				State: workload.StateDeleted,
			},
			expected: workload.Status{
				State: workload.StateDeleted,
			},
		},
		{
			name: "update both port and state",
			status: workload.Status{
				State: workload.StateDeleted,
				Port:  1337,
			},
			expected: workload.Status{
				State: workload.StateDeleted,
				Port:  1337,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := workload.NewStore()
			store.Update("abc", tt.status)
			acutal := store.Get("abc")
			require.Equal(t, tt.expected, *acutal)
		})
	}
}

func TestStatusStoreDelete(t *testing.T) {
	store := workload.NewStore()
	store.Update("abc", workload.Status{
		State: workload.StateDeleted,
		Port:  1337,
	})
	store.Del("abc")
	require.Nil(t, store.Get("abc"))
}

func TestStatusStoreView(t *testing.T) {
	var (
		store  = workload.NewStore()
		status = workload.Status{
			State: workload.StateDeleted,
			Port:  1337,
		}
		expected = map[string]workload.Status{
			"abc": status,
		}
	)
	store.Update("abc", status)

	view := store.View()
	require.Equal(t, expected, view)

	// now make sure modifications will not affect
	// the backing map of the store

	delete(view, "abc")

	require.Equal(t, expected, store.View())
}
