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
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spacechunks/explorer/internal/image"
	imgtestdata "github.com/spacechunks/explorer/internal/image/testdata"
	"github.com/spacechunks/explorer/internal/tarhelper"
	"github.com/stretchr/testify/require"
)

func TestAppendLayer(t *testing.T) {
	base := imgtestdata.Image(t)

	actual, err := image.AppendLayer(base, "./testdata/layertest")
	require.NoError(t, err)

	layers, err := actual.Layers()
	require.NoError(t, err)

	want := []string{
		"opt",
		"opt/paper",
		"opt/paper/file2",
		"opt/paper/testdir",
		"opt/paper/testdir/file1",
	}

	tmpDir := t.TempDir()

	for _, l := range layers {
		rc, err := l.Compressed()
		require.NoError(t, err)

		defer rc.Close()

		h, err := l.Digest()
		require.NoError(t, err)

		dest := filepath.Join(tmpDir, h.Hex)

		paths, err := tarhelper.Untar(rc, dest)
		require.NoError(t, err)

		got := make([]string, 0)
		for _, p := range paths {
			got = append(got, strings.ReplaceAll(p, dest+"/", ""))
		}

		if d := cmp.Diff(want, got); d == "" {
			return
		} else {
			t.Logf("diff -want +got\n%s", d)
		}
	}
}
