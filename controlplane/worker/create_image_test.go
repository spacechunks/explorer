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
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/riverqueue/river"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/chunk"
	imgtestdata "github.com/spacechunks/explorer/controlplane/image/testdata"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/spacechunks/explorer/controlplane/worker"
	"github.com/spacechunks/explorer/internal/mock"
	"github.com/spacechunks/explorer/test/fixture"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCreateImageWorker(t *testing.T) {
	var (
		ctx        = context.Background()
		imgService = mock.NewMockImageService(t)
		blobStore  = mock.NewMockBlobStore(t)
		repo       = mock.NewMockChunkRepository(t)
		jobClient  = mock.NewMockJobClient(t)
	)

	ctx = context.WithValue(ctx, worker.ContextKeyImageService, imgService)
	ctx = context.WithValue(ctx, worker.ContextKeyBlobStore, blobStore)
	ctx = context.WithValue(ctx, worker.ContextKeyChunkRepository, repo)
	ctx = context.WithValue(ctx, worker.ContextKeyJobClient, jobClient)

	var (
		registry         = "example.com"
		baseImgRef       = "example.com/base:latest"
		c                = fixture.Chunk()
		flavor           = c.Flavors[0]
		flavorVersion    = c.Flavors[0].Versions[0]
		checkpointImgRef = fmt.Sprintf("%s/%s/%s:%s", registry, c.Name, flavor.Name, flavorVersion.Version)
	)

	hashToPath := make(map[string]string, len(flavorVersion.FileHashes))
	for _, fh := range flavorVersion.FileHashes {
		hashToPath[fh.Hash] = fh.Path
	}

	sl := slices.Collect(maps.Keys(hashToPath))

	sort.Slice(sl, func(i, j int) bool {
		return strings.Compare(sl[i], sl[j]) < 0
	})

	fileData := make(map[string][]byte)
	for _, fh := range flavorVersion.FileHashes {
		fileData[fh.Path] = []byte("some-data")
	}

	var objs []blob.Object
	for _, fh := range flavorVersion.FileHashes {
		objs = append(objs, blob.Object{
			Hash: fh.Hash,
			Data: []byte("some-data"),
		})
	}

	imgService.EXPECT().
		Pull(mocky.Anything, baseImgRef).
		Return(imgtestdata.Image(t), nil)

	repo.EXPECT().
		FlavorVersionByID(mocky.Anything, flavorVersion.ID).
		Return(flavorVersion, nil)

	blobStore.EXPECT().
		Get(mocky.Anything, sl).
		Return(objs, nil)

	imgService.EXPECT().
		Push(mocky.Anything, mocky.Anything, checkpointImgRef).
		Return(nil)

	jobClient.EXPECT().
		InsertJob(
			mocky.Anything,
			flavorVersion.ID,
			string(chunk.BuildStatusBuildCheckpoint),
			job.CreateCheckpoint{BaseImage: checkpointImgRef},
		).
		Return(nil)

	w := worker.CreateImageWorker{}

	riverJob := &river.Job[job.CreateImage]{
		Args: job.CreateImage{
			FlavorVersionID: flavorVersion.ID,
			BaseImage:       baseImgRef,
			OCIRegistry:     registry,
		},
	}

	require.NoError(t, w.Work(ctx, riverJob))
}
