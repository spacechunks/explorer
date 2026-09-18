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

package chunk

import (
	"context"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/internal/file"
	"github.com/spacechunks/explorer/internal/mock"
	"github.com/spacechunks/explorer/test"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestComputeFileHashes(t *testing.T) {
	var (
		ctx     = context.Background()
		tarData = test.CreateTarGz(t, map[string]string{
			"server.properties":   "bla",
			"plugins/config.yaml": "lol",
		})
		mockStore = mock.NewMockBlobS3Store(t)
		versionID = "version-id"
		expected  = []file.Hash{
			{
				Path: "server.properties",
				Hash: "038d12a21e489bb2",
			},
			{
				Path: "plugins/config.yaml",
				Hash: "7b75e34aa5423334",
			},
		}
	)

	mockStore.EXPECT().
		WriteTo(mocky.Anything, blob.ChangeSetKey(versionID), mocky.Anything).
		RunAndReturn(func(ctx context.Context, key string, w io.Writer) error {
			_, err := w.Write(tarData)
			require.NoError(t, err)
			return nil
		})

	s := svc{
		s3Store: mockStore,
	}

	actual, err := s.computeFileHashes(ctx, versionID)
	require.NoError(t, err)

	file.SortHashes(expected)
	file.SortHashes(actual)

	if d := cmp.Diff(expected, actual); d != "" {
		t.Fatalf("mismatch (-want +got):\n%s", d)
	}
}
