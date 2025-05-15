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

package worker

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/riverqueue/river"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/image"
	"github.com/spacechunks/explorer/controlplane/job"
)

type CreateImageWorker struct {
	river.WorkerDefaults[job.CreateImage]
}

func (w *CreateImageWorker) Work(ctx context.Context, riverJob *river.Job[job.CreateImage]) error {
	if err := riverJob.Args.Validate(); err != nil {
		return fmt.Errorf("validate args: %w", err)
	}

	repo, ok := ctx.Value(ContextKeyChunkRepository).(chunk.Repository)
	if !ok {
		return errors.New("repo not found in context")
	}

	blobStore, ok := ctx.Value(ContextKeyBlobStore).(blob.Store)
	if !ok {
		return errors.New("blob store not found in context")
	}

	imgService, ok := ctx.Value(ContextKeyImageService).(image.Service)
	if !ok {
		return errors.New("img service not found in context")
	}

	jobClient, ok := ctx.Value(ContextKeyJobClient).(job.Client)
	if !ok {
		return errors.New("img service not found in context")
	}

	baseImg, err := imgService.Pull(ctx, riverJob.Args.BaseImage)
	if err != nil {
		return fmt.Errorf("pull image: %w", err)
	}

	version, err := repo.FlavorVersionByID(ctx, riverJob.Args.FlavorVersionID)
	if err != nil {
		return fmt.Errorf("flavor version: %w", err)
	}

	hashToPath := make(map[string]string, len(version.FileHashes))

	for _, fh := range version.FileHashes {
		hashToPath[fh.Hash] = fh.Path
	}

	// needed for testing to have a consistent order
	sl := slices.Collect(maps.Keys(hashToPath))
	sort.Slice(sl, func(i, j int) bool {
		return strings.Compare(sl[i], sl[j]) < 0
	})

	objs, err := blobStore.Get(ctx, sl)
	if err != nil {
		return fmt.Errorf("get objs: %w", err)
	}

	f := make(map[string][]byte, len(objs))
	for _, obj := range objs {
		f[hashToPath[obj.Hash]] = obj.Data
	}

	layer, err := image.LayerFromFiles(f)
	if err != nil {
		return fmt.Errorf("layer from files: %w", err)
	}

	img, err := mutate.AppendLayers(baseImg, layer)
	if err != nil {
		return fmt.Errorf("append layer: %w", err)
	}

	// FIXME: if we implement users add user id to ref
	//        => <registry>/<userID>/<flavor-version-id>:<base|checkpoint>
	ref := fmt.Sprintf("%s/%s:base", riverJob.Args.OCIRegistry, riverJob.Args.FlavorVersionID)

	if err := imgService.Push(ctx, img, ref); err != nil {
		return fmt.Errorf("push image: %w", err)
	}

	if err := jobClient.InsertJob(
		ctx,
		riverJob.Args.FlavorVersionID,
		string(chunk.BuildStatusBuildCheckpoint),
		job.CreateCheckpoint{
			BaseImage: ref,
		}); err != nil {
		return fmt.Errorf("insert create checkpoint job: %w", err)
	}

	return nil
}
