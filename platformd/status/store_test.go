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

package status_test

import (
	"testing"

	"github.com/spacechunks/explorer/platformd/status"
	"github.com/stretchr/testify/require"
)

func TestStatusStoreUpdate(t *testing.T) {
	tests := []struct {
		name     string
		status   status.Status
		expected status.Status
	}{
		{
			name: "workload update port only",
			status: status.Status{
				WorkloadStatus: &status.WorkloadStatus{
					Port: 1337,
				},
			},
			expected: status.Status{
				WorkloadStatus: &status.WorkloadStatus{
					Port: 1337,
				},
			},
		},
		{
			name: "workload update state only",
			status: status.Status{
				WorkloadStatus: &status.WorkloadStatus{
					State: status.WorkloadStateDeleted,
				},
			},
			expected: status.Status{
				WorkloadStatus: &status.WorkloadStatus{
					State: status.WorkloadStateDeleted,
				},
			},
		},
		{
			name: "workload update both port and state",
			status: status.Status{
				WorkloadStatus: &status.WorkloadStatus{
					State: status.WorkloadStateDeleted,
					Port:  1337,
				},
			},
			expected: status.Status{
				WorkloadStatus: &status.WorkloadStatus{
					State: status.WorkloadStateDeleted,
					Port:  1337,
				},
			},
		},
		// TODO: add tests for checkpoint status
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := status.NewMemStore()
			store.Update("abc", tt.status)
			acutal := store.Get("abc")
			require.Equal(t, tt.expected, *acutal)
		})
	}
}

func TestStatusStoreDelete(t *testing.T) {
	store := status.NewMemStore()
	store.Update("abc", status.Status{
		WorkloadStatus: &status.WorkloadStatus{
			State: status.WorkloadStateDeleted,
			Port:  1337,
		},
	})
	store.Del("abc")
	require.Nil(t, store.Get("abc"))
}

func TestStatusStoreView(t *testing.T) {
	var (
		store = status.NewMemStore()
		st    = status.Status{
			WorkloadStatus: &status.WorkloadStatus{
				State: status.WorkloadStateDeleted,
				Port:  1337,
			},
		}
		expected = map[string]status.Status{
			"abc": st,
		}
	)
	store.Update("abc", st)

	view := store.View()
	require.Equal(t, expected, view)

	// now make sure modifications will not affect
	// the backing map of the store

	delete(view, "abc")

	require.Equal(t, expected, store.View())
}
