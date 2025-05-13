package image

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"

	ociv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

func LayerFromFiles(files map[string][]byte) (ociv1.Layer, error) {
	var (
		w    bytes.Buffer
		tarw = tar.NewWriter(&w)
	)

	for path, data := range files {
		hdr := &tar.Header{
			Name:     path,
			Typeflag: tar.TypeReg,
			Mode:     0777, // FIXME: pass file mode at some point
			Size:     int64(len(data)),
		}

		if err := tarw.WriteHeader(hdr); err != nil {
			return nil, fmt.Errorf("write hdr: %w", err)
		}

		if _, err := tarw.Write(data); err != nil {
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
