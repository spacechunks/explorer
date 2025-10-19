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
	"sort"
	"strings"
	"testing"

	"github.com/spacechunks/explorer/internal/tarhelper"
	"github.com/spacechunks/explorer/internal/tarhelper/testdata"
	"github.com/stretchr/testify/require"
)

func TestUntar(t *testing.T) {
	dir := t.TempDir()

	want := []string{
		dir + "/dir/dir1/test2",
		dir + "/dir/test1",
	}

	got, err := tarhelper.Untar(bytes.NewReader(testdata.TarFile), dir)
	require.NoError(t, err)

	sort.Slice(want, func(i, j int) bool {
		return strings.Compare(want[i], want[j]) < 0
	})

	sort.Slice(got, func(i, j int) bool {
		return strings.Compare(got[i], got[j]) < 0
	})

	require.Equal(t, want, got)
}
