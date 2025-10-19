package image

import (
	"archive/tar"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	ociv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// AppendLayer appends a layer based on the directory tree dir to the given
// base image. note that an intermediate tarball containing the layer will
// be created at ../$dir/__layer.tar. any file located at this destination is
// going to be overridden.
func AppendLayer(base ociv1.Image, dir string) (ociv1.Image, error) {
	rt, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open root: %w", err)
	}

	layer, err := layerFromFS(filepath.Join(filepath.Dir(dir), "__layer.tar"), rt.FS())
	if err != nil {
		return nil, fmt.Errorf("layer from files: %w", err)
	}

	img, err := mutate.AppendLayers(base, layer)
	if err != nil {
		return nil, fmt.Errorf("append layer: %w", err)
	}

	return img, nil
}

// layerFromFS creates a tarball file at path dest based on fs.
func layerFromFS(dest string, fs fs.FS) (ociv1.Layer, error) {
	f, err := os.Create(dest)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}

	defer f.Close()

	tarw := tar.NewWriter(f)

	if err := tarw.AddFS(fs); err != nil {
		return nil, fmt.Errorf("add fs: %w", err)
	}

	if err := tarw.Close(); err != nil {
		return nil, fmt.Errorf("tar close: %w", err)
	}

	return tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return os.Open(dest)
	})
}
