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

package tarhelper_test

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spacechunks/explorer/internal/tarhelper"
	"github.com/spacechunks/explorer/internal/tarhelper/testdata"
	"github.com/stretchr/testify/require"
)

func TestTarFiles(t *testing.T) {
	dir := t.TempDir()
	dest := dir + "/tarfs.tar.gz"

	files := make([]*os.File, 0)
	err := filepath.Walk("./testdata/dir", func(path string, info os.FileInfo, _ error) error {
		if info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}

		files = append(files, f)
		return nil
	})
	require.NoError(t, err)

	err = tarhelper.TarFiles("./testdata/dir", files, dest)
	require.NoError(t, err)

	f, err := os.Open(dest)
	require.NoError(t, err)

	want := []string{
		dir + "/dir1/test2",
		dir + "/test1",
	}

	got, err := tarhelper.Untar(f, filepath.Dir(dest))
	require.NoError(t, err)

	checkPaths(t, want, got)
}

func TestUntar(t *testing.T) {
	dir := t.TempDir()

	want := []string{
		dir + "/dir/dir1/test2",
		dir + "/dir/test1",
	}

	got, err := tarhelper.Untar(bytes.NewReader(testdata.TarFile), dir)
	require.NoError(t, err)

	checkPaths(t, want, got)
}

func checkPaths(t *testing.T, want []string, got []string) {
	sort.Slice(want, func(i, j int) bool {
		return strings.Compare(want[i], want[j]) < 0
	})

	sort.Slice(got, func(i, j int) bool {
		return strings.Compare(got[i], got[j]) < 0
	})

	require.Equal(t, want, got)
}
