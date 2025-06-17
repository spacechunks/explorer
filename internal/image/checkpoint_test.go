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
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	ociv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/compare"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/spacechunks/explorer/internal/image"
	"github.com/spacechunks/explorer/internal/image/testdata"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/stretchr/testify/require"
)

func TestFromCheckpoint(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		err         bool
	}{
		{
			name: "works",
			annotations: map[string]string{
				image.CheckpointAnnotation: "payload",
			},
		},
		{
			name: "annotations differ",
			annotations: map[string]string{
				"foo": "bar",
			},
			err: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				svc = image.NewService(
					slog.New(slog.NewTextHandler(os.Stdout, nil)),
					fixture.OCIRegsitryUser,
					fixture.OCIRegistryPass,
					t.TempDir(),
				)
				addr           = fixture.RunRegistry(t)
				createdAt      = time.Now()
				createdBy      = "test"
				arch           = "arm64"
				actualRefStr   = addr + "/actual"
				expectedRefStr = addr + "/expected"
			)

			path := t.TempDir() + "/file.tar.gz"

			err := os.WriteFile(path, testdata.RawImage, 0777)
			require.NoError(t, err)

			expectedImg := buildExpectedImg(t, arch, createdBy, createdAt, tt.annotations)

			actualImg, err := image.FromCheckpoint(path, arch, createdBy, createdAt)
			require.NoError(t, err)

			// there is no nice way of comparing two ociv1.Images that contain
			// a streaming layer. so we just push them to the registry
			// and use remote.Image to pull them again. comparing the pulled
			// images works.
			pushPull := func(refStr string, in ociv1.Image) ociv1.Image {
				err = svc.Push(ctx, in, refStr)
				require.NoError(t, err)

				err = svc.Push(ctx, in, refStr)
				require.NoError(t, err)

				ref, err := name.ParseReference(refStr)
				require.NoError(t, err)

				auth := image.Auth{
					Username: fixture.OCIRegsitryUser,
					Password: fixture.OCIRegistryPass,
				}

				pulled, err := remote.Image(ref, remote.WithAuth(auth))
				require.NoError(t, err)

				return pulled
			}

			expected := pushPull(expectedRefStr, expectedImg)
			actual := pushPull(actualRefStr, actualImg)

			err = compare.Images(expected, actual)
			if tt.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func buildExpectedImg(
	t *testing.T,
	arch string,
	createdBy string,
	createdAt time.Time,
	annotations map[string]string,
) ociv1.Image {
	var (
		created = ociv1.Time{
			Time: createdAt,
		}
	)

	cfg, err := empty.Image.ConfigFile()
	require.NoError(t, err)

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
	require.NoError(t, err)

	img = mutate.ConfigMediaType(img, types.OCIConfigJSON)

	img, err = mutate.AppendLayers(
		img,
		stream.NewLayer(
			io.NopCloser(bytes.NewReader(testdata.RawImage)),
			stream.WithMediaType(types.OCILayer)),
	)
	require.NoError(t, err)

	img = mutate.Annotations(img, annotations).(ociv1.Image)
	img = mutate.MediaType(img, types.OCIManifestSchema1)

	return img
}
