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
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/riverqueue/river"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/file"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/spacechunks/explorer/internal/image"
)

type CreateImageWorker struct {
	river.WorkerDefaults[job.CreateImage]
	repo       chunk.Repository
	blobStore  blob.Store
	imgService image.Service
	jobClient  job.Client
}

func NewCreateImageWorker(
	repo chunk.Repository,
	blobStore blob.Store,
	imgSvc image.Service,
	jobClient job.Client,
) *CreateImageWorker {
	return &CreateImageWorker{
		repo:       repo,
		blobStore:  blobStore,
		imgService: imgSvc,
		jobClient:  jobClient,
	}
}

func (w *CreateImageWorker) Work(ctx context.Context, riverJob *river.Job[job.CreateImage]) error {
	if err := riverJob.Args.Validate(); err != nil {
		return fmt.Errorf("validate args: %w", err)
	}

	baseImg, err := w.imgService.Pull(ctx, riverJob.Args.BaseImage)
	if err != nil {
		return fmt.Errorf("pull image: %w", err)
	}

	version, err := w.repo.FlavorVersionByID(ctx, riverJob.Args.FlavorVersionID)
	if err != nil {
		return fmt.Errorf("flavor version: %w", err)
	}

	hashToPath := make(map[string]string, len(version.FileHashes))

	for _, fh := range version.FileHashes {
		hashToPath[fh.Hash] = fh.Path
	}

	// needed for testing to have a consistent order
	hashes := slices.Collect(maps.Keys(hashToPath))
	sort.Slice(hashes, func(i, j int) bool {
		return strings.Compare(hashes[i], hashes[j]) < 0
	})

	objs, err := w.blobStore.Get(ctx, hashes)
	if err != nil {
		return fmt.Errorf("get objs: %w", err)
	}

	f := make([]file.Object, 0, len(objs))
	for _, obj := range objs {
		f = append(f, file.Object{
			Path: hashToPath[obj.Hash],
			Data: obj.Data,
		})
	}

	img, err := image.AppendLayer(baseImg, f)
	if err != nil {
		return fmt.Errorf("append layer: %w", err)
	}

	// FIXME: if we implement users add user id to ref
	//        => <registry>/<userID>/<flavor-version-id>:<base|checkpoint>
	ref := fmt.Sprintf("%s/%s:base", riverJob.Args.OCIRegistry, riverJob.Args.FlavorVersionID)

	if err := w.imgService.Push(ctx, img, ref); err != nil {
		return fmt.Errorf("push image: %w", err)
	}

	if err := w.jobClient.InsertJob(
		ctx,
		riverJob.Args.FlavorVersionID,
		string(chunk.BuildStatusBuildCheckpoint),
		job.CreateCheckpoint{
			FlavorVersionID: riverJob.Args.FlavorVersionID,
			BaseImageURL:    ref,
		}); err != nil {
		return fmt.Errorf("insert create checkpoint job: %w", err)
	}

	return nil
}
