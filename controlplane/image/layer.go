package image

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"

	ociv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spacechunks/explorer/controlplane/file"
)

func LayerFromFiles(files []file.Object) (ociv1.Layer, error) {
	var (
		w    bytes.Buffer
		tarw = tar.NewWriter(&w)
	)

	for _, f := range files {
		hdr := &tar.Header{
			Name:     f.Path,
			Typeflag: tar.TypeReg,
			Mode:     0777, // FIXME: pass file mode at some point
			Size:     int64(len(f.Data)),
		}

		if err := tarw.WriteHeader(hdr); err != nil {
			return nil, fmt.Errorf("write hdr: %w", err)
		}

		if _, err := tarw.Write(f.Data); err != nil {
			return nil, fmt.Errorf("write file content: %w", err)
		}
	}

	if err := tarw.Close(); err != nil {
		return nil, fmt.Errorf("tar close: %w", err)
	}

	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(w.Bytes())), nil
	})

	if err != nil {
		return nil, fmt.Errorf("create layer: %w", err)
	}

	return layer, nil
}

func AppendLayer(base ociv1.Image, files []file.Object) (ociv1.Image, error) {
	layer, err := LayerFromFiles(files)
	if err != nil {
		return nil, fmt.Errorf("layer from files: %w", err)
	}

	img, err := mutate.AppendLayers(base, layer)
	if err != nil {
		return nil, fmt.Errorf("append layer: %w", err)
	}

	return img, nil
}
