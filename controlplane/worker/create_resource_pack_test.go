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

package worker_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/spacechunks/explorer/controlplane/worker"
	"github.com/spacechunks/explorer/internal/mock"
	"github.com/spacechunks/explorer/test"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	itemTemplate = `
{
  "model": {
    "type": "minecraft:model",
    "model": "spacechunks:item/explorer/chunk_viewer/thumbnails/{chunk_id}"
  }
}
`
	modelTemplate = `
{
  "parent": "spacechunks:item/explorer/chunk_viewer/flat_ui_element",
  "textures": {
    "layer0": "spacechunks:item/explorer/chunk_viewer/thumbnails/{chunk_id}"
  }
}
`
)

func TestBuildResourcePackWorkerSuccessfullyBuildsPack(t *testing.T) {
	var (
		ctx      = context.Background()
		mockS3   = mock.NewMockBlobS3Store(t)
		mockRepo = mock.NewMockChunkRepository(t)
		logger   = slog.New(slog.NewTextHandler(os.Stdout, nil))
		cfg      = worker.CreateResourcePackWorkerConfig{
			WorkingDir:        t.TempDir(),
			PackTemplateKey:   "explorer/pack_template.zip",
			ItemTemplatePath:  "assets/spc/items/test/_template.json",
			ModelTemplatePath: "assets/spc/models/item/test/_template.json",
			ItemDir:           "assets/spc/items/test",
			ModelDir:          "assets/spc/models/item/test",
			TextureDir:        "assets/spc/textures/item/test",
		}

		templatePack, _ = test.CreateResourcePackZip(t, map[string]string{
			"assets/spc/items/test/_template.json":       itemTemplate,
			"assets/spc/models/item/test/_template.json": modelTemplate,
		})

		_, expectedHash = test.CreateResourcePackZip(t, map[string]string{
			// items
			"assets/spc/items/test/_template.json": itemTemplate,
			"assets/spc/items/test/id1.json":       strings.ReplaceAll(itemTemplate, "{chunk_id}", "id1"),
			"assets/spc/items/test/id2.json":       strings.ReplaceAll(itemTemplate, "{chunk_id}", "id2"),

			// models
			"assets/spc/models/item/test/_template.json": modelTemplate,
			"assets/spc/models/item/test/id1.json":       strings.ReplaceAll(modelTemplate, "{chunk_id}", "id1"),
			"assets/spc/models/item/test/id2.json":       strings.ReplaceAll(modelTemplate, "{chunk_id}", "id2"),

			// textures
			"assets/spc/textures/item/test/id1.png": "id1.png",
			"assets/spc/textures/item/test/id2.png": "id2.png",
		})
	)

	mockS3.EXPECT().
		WriteTo(mocky.Anything, cfg.PackTemplateKey, mocky.Anything).
		Run(func(ctx context.Context, key string, w io.Writer) {
			_, err := io.Copy(w, templatePack)
			require.NoError(t, err)
		}).
		Return(nil)

	mockS3.EXPECT().
		WriteTo(mocky.Anything, blob.CASKeyPrefix+"/"+"imghash1", mocky.Anything).
		Run(func(ctx context.Context, key string, w io.Writer) {
			_, err := io.WriteString(w, "id1.png")
			require.NoError(t, err)
		}).
		Return(nil)

	mockS3.EXPECT().
		WriteTo(mocky.Anything, blob.CASKeyPrefix+"/"+"imghash2", mocky.Anything).
		Run(func(ctx context.Context, key string, w io.Writer) {
			_, err := io.WriteString(w, "id2.png")
			require.NoError(t, err)
		}).
		Return(nil)

	mockS3.EXPECT().SimplePut(
		mocky.Anything, "explorer/latest.zip", mocky.Anything, map[string]string{
			"sha1": expectedHash,
		}).
		Return(nil)

	mockRepo.EXPECT().
		AllChunkThumbnailHashes(mocky.Anything).
		Return(map[string]string{
			"id1": "imghash1",
			"id2": "imghash2",
		}, nil)

	w := worker.NewCreateResourcePackWorker(
		logger,
		mockS3,
		mockRepo,
		cfg,
	)

	riverJob := &river.Job[job.CreateResourcePack]{
		JobRow: &rivertype.JobRow{
			ID: 1337,
		},
	}

	err := w.Work(ctx, riverJob)
	require.NoError(t, err)
}
