package image_test

import (
	"bytes"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spacechunks/platform/internal/image"
	"github.com/spacechunks/platform/internal/image/testdata"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
)

// use simple test, because different layer configurations are
// generated by docker build. see testdata/Dockerfile.unpack
func TestUnpackDir(t *testing.T) {
	testImgOpener := func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(testdata.UnpackImage)), nil
	}
	img, err := tarball.Image(testImgOpener, nil)
	if err != nil {
		t.Fatalf("read img: %v", err)
	}
	expected := []image.File{
		{
			AbsPath: "a/",
			RelPath: "/",
			Dir:     true,
		},
		{
			AbsPath: "a/b/",
			RelPath: "/b/",
			Dir:     true,
		},
		{
			AbsPath: "a/b/file2",
			RelPath: "/b/file2",
			Content: []byte("changed\n"),
			Size:    8,
		},
		{
			AbsPath: "a/b/c/",
			RelPath: "/b/c/",
			Dir:     true,
		},
		{
			AbsPath: "a/b/c/file3",
			RelPath: "/b/c/file3",
			Content: []byte("file3\n"),
			Size:    6,
		},
	}
	files, err := image.UnpackDir(img, "a")
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}
	assert.ElementsMatch(t, expected, files)
}