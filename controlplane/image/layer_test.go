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
	"testing"

	"github.com/spacechunks/explorer/controlplane/file"
	"github.com/spacechunks/explorer/controlplane/image"
	imgtestdata "github.com/spacechunks/explorer/controlplane/image/testdata"
	"github.com/stretchr/testify/require"
)

func TestAppendLayer(t *testing.T) {
	layerFiles := []file.Object{
		{
			Path: "/opt/paper/test1",
			Data: []byte("test1"),
		},
		{
			Path: "/opt/paper/test",
			Data: []byte("helll"),
		},
		{
			Path: "/opt/paper/dir/test2",
			Data: []byte("test2"),
		},
	}

	base := imgtestdata.Image(t)

	actual, err := image.AppendLayer(base, layerFiles)
	require.NoError(t, err)

	layer, err := image.LayerFromFiles(layerFiles)
	require.NoError(t, err)

	dig, err := layer.Digest()
	require.NoError(t, err)

	// throws an error if digest is not found
	_, err = actual.LayerByDigest(dig)
	require.NoError(t, err)
}
