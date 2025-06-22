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

package image

import (
	"fmt"
	"os"
	"time"

	ociv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// CheckpointAnnotation is used by crio to identify an image containing a checkpoint.
const CheckpointAnnotation = "io.kubernetes.cri-o.annotations.checkpoint.name"

// FromCheckpoint creates a crio-compatible checkpoint image. path needs to point to a tarball which contains
// the checkpoint data. internally, a single layer will be created that consists of the checkpoint tarball.
// the layer will be streamed when pushed to a registry, due to checkpoints being potentially very large files.
// this also means that the tarball should not be removed before this image has been pushed.
func FromCheckpoint(path string, arch string, createdBy string, createdAt time.Time) (ociv1.Image, error) {
	f, err := os.OpenFile(path, os.O_RDONLY, 0777)
	if err != nil {
		return nil, err
	}

	created := ociv1.Time{
		Time: createdAt,
	}

	cfg, err := empty.Image.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("read image config: %w", err)
	}

	cfg.Architecture = arch
	cfg.OS = "linux"
	cfg.Created = created
	cfg.History = []ociv1.History{
		{
			Created:   created,
			CreatedBy: createdBy,
		},
	}

	img, err := mutate.ConfigFile(empty.Image, cfg)
	if err != nil {
		return nil, fmt.Errorf("write image config: %w", err)
	}

	img = mutate.ConfigMediaType(img, types.OCIConfigJSON)

	// use a streaming layer here, because checkpoints
	// will be very large.
	img, err = mutate.AppendLayers(img, stream.NewLayer(f, stream.WithMediaType(types.OCILayer)))
	if err != nil {
		return nil, fmt.Errorf("append layer: %w", err)
	}

	img = mutate.Annotations(img, map[string]string{
		CheckpointAnnotation: "payload",
	}).(ociv1.Image)

	img = mutate.MediaType(img, types.OCIManifestSchema1)

	return img, nil
}
