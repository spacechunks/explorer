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

package image_test

import (
	"archive/tar"
	"io"
	"testing"

	"github.com/spacechunks/explorer/controlplane/image"
	"github.com/stretchr/testify/require"
)

func TestAppendLayerFromFiles(t *testing.T) {
	expected := map[string][]byte{
		"/opt/paper/test1":     []byte("test1"),
		"/opt/paper/test":      []byte("helll"),
		"/opt/paper/dir/test2": []byte("test2"),
	}

	layer, err := image.LayerFromFiles(expected)
	require.NoError(t, err)

	r, err := layer.Uncompressed()
	require.NoError(t, err)

	actual := make(map[string][]byte)

	tarr := tar.NewReader(r)

	for {
		hdr, err := tarr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		actual[hdr.Name] = make([]byte, hdr.Size)
		_, err = io.ReadFull(tarr, actual[hdr.Name])
		require.NoError(t, err)
	}

	require.Equal(t, expected, actual)
}
