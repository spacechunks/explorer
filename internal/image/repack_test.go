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
	"bytes"
	"io"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spacechunks/explorer/internal/image"
	"github.com/spacechunks/explorer/internal/image/testdata"
	"github.com/stretchr/testify/assert"
)

func TestRepack(t *testing.T) {
	testImgOpener := func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(testdata.RepackImage)), nil
	}
	src, err := tarball.Image(testImgOpener, nil)
	if err != nil {
		t.Fatalf("read img: %v", err)
	}
	expected := []image.File{
		{
			AbsPath:      "a/",
			StrippedPath: "/",
			Dir:          true,
		},
		{
			AbsPath:      "a/file1",
			StrippedPath: "/file1",
			Content:      []byte("file1 repacked\n"),
			Size:         15,
		},
		{
			AbsPath:      "a/b/",
			StrippedPath: "/b/",
			Dir:          true,
		},
		{
			AbsPath:      "a/b/file2",
			StrippedPath: "/b/file2",
			Content:      []byte("file2 repacked\n"),
			Size:         15,
		},
		{
			AbsPath:      "a/b/file4",
			StrippedPath: "/b/file4",
			Content:      []byte("file4 repacked\n"),
			Size:         15,
		},
		{
			AbsPath:      "a/b/c/",
			StrippedPath: "/b/c/",
			Dir:          true,
		},
		{
			AbsPath:      "a/b/c/file3",
			StrippedPath: "/b/c/file3",
			Content:      []byte("abc\n"),
			Size:         4,
		},
		{
			AbsPath:      "a/d/",
			StrippedPath: "/d/",
			Dir:          true,
		},
		{
			AbsPath:      "a/d/file",
			StrippedPath: "/d/file",
			Content:      []byte("repacked file\n"),
			Size:         14,
		},
	}

	img, err := image.Repack(src, "a", "overlay")
	if err != nil {
		t.Fatalf("repack: %v", err)
	}

	files, err := image.UnpackDir(img, "a")
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}
	assert.ElementsMatch(t, expected, files)
}
