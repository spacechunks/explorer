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
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/riverqueue/river"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/spacechunks/explorer/controlplane/resource"
	"github.com/spacechunks/explorer/internal/file"
	"github.com/spacechunks/explorer/internal/image"
	"github.com/spacechunks/explorer/internal/tarhelper"
)

type CreateImageWorker struct {
	logger *slog.Logger
	river.WorkerDefaults[job.CreateImage]
	repo          chunk.Repository
	store         blob.S3Store
	imgService    image.Service
	jobClient     job.Client
	imagePlatform string
}

func NewCreateImageWorker(
	logger *slog.Logger,
	repo chunk.Repository,
	imgSvc image.Service,
	jobClient job.Client,
	store blob.S3Store,
	imagePlatform string,
) *CreateImageWorker {
	return &CreateImageWorker{
		logger:        logger,
		repo:          repo,
		store:         store,
		imgService:    imgSvc,
		jobClient:     jobClient,
		imagePlatform: imagePlatform,
	}
}

func (w *CreateImageWorker) Work(ctx context.Context, riverJob *river.Job[job.CreateImage]) (ret error) {
	defer func() {
		if ret == nil {
			return
		}

		// we only want to update the job to failed
		// once we exhausted all attempts.
		if riverJob.Attempt < riverJob.MaxAttempts {
			return
		}

		if err := w.repo.UpdateFlavorVersionBuildStatus(
			ctx,
			riverJob.Args.FlavorVersionID,
			resource.BuildStatusBuildImageFailed,
		); err != nil {
			w.logger.ErrorContext(ctx, "failed to update flavor version build status", "err", err)
		}
	}()

	if err := riverJob.Args.Validate(); err != nil {
		return fmt.Errorf("validate args: %w", err)
	}

	baseImg, err := w.imgService.Pull(ctx, riverJob.Args.BaseImage, w.imagePlatform)
	if err != nil {
		return fmt.Errorf("pull image: %w", err)
	}

	version, err := w.repo.FlavorVersionByID(ctx, riverJob.Args.FlavorVersionID)
	if err != nil {
		return fmt.Errorf("flavor version: %w", err)
	}

	var (
		rootDir       = fmt.Sprintf("/tmp/%d", riverJob.ID)
		filesDir      = rootDir + "/files"
		serverRootDir = filesDir + "/opt/paper"
	)

	defer func() {
		if err := os.RemoveAll(rootDir); err != nil {
			w.logger.ErrorContext(
				ctx,
				"failed to remove files",
				"flavor_version_id", riverJob.Args.FlavorVersionID,
				"river_job_id", riverJob.ID,
				"err", err,
			)
		}
	}()

	tb, err := os.Create("changeset.tar.gz")
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}

	defer tb.Close()

	if err := w.store.WriteTo(ctx, blob.ChangeSetKey(version.ID), tb); err != nil {
		return fmt.Errorf("write tarball: %w", err)
	}

	if _, err := tb.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek: %w", err)
	}

	// the server root dir we use in our base image is /opt/paper
	// so all files should be located right there.
	paths, err := tarhelper.Untar(tb, serverRootDir)
	if err != nil {
		return fmt.Errorf("untar files: %w", err)
	}

	if err := w.upload(ctx, paths); err != nil {
		return fmt.Errorf("upload files: %w", err)
	}

	if err := w.downloadMissing(ctx, serverRootDir, version.FileHashes, paths); err != nil {
		return fmt.Errorf("download missing: %w", err)
	}

	// TODO: adjust configs

	// it is VERY important we specify the parent of the server root directory,
	// because only paths starting INSIDE the passed directory are preserved.
	// so, in our case we specify files/opt/paper to keep the /opt/paper prefix.
	img, err := image.AppendLayer(baseImg, filesDir)
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
		string(resource.BuildStatusBuildCheckpoint),
		job.CreateCheckpoint{
			FlavorVersionID: riverJob.Args.FlavorVersionID,
			BaseImageURL:    ref,
		}); err != nil {
		return fmt.Errorf("insert create checkpoint job: %w", err)
	}

	return nil
}

func (w *CreateImageWorker) upload(ctx context.Context, filePaths []string) error {
	objs := make([]blob.Object, 0)

	for _, p := range filePaths {
		obj, err := blob.NewFromFile(p)
		if err != nil {
			return fmt.Errorf("new object: %w", err)
		}

		objs = append(objs, obj)
	}

	// store will check if there are any duplicates
	if err := w.store.Put(ctx, blob.CASKeyPrefix, objs); err != nil {
		return fmt.Errorf("upload objects: %w", err)
	}

	return nil
}

func (w *CreateImageWorker) downloadMissing(ctx context.Context, dest string, all []file.Hash, have []string) error {
	var (
		want    = make([]file.Hash, 0)
		cleaned = make(map[string]struct{}, len(have))
	)

	for _, s := range have {
		// strip out /tmp/123/opt/paper/ from /tmp/123/opt/paper/plugins/test.jar
		// so we are left with plugins/test.jar. this is needed, so we can do
		// a simple == check when comparing paths later.
		cleaned[strings.Replace(s, dest+"/", "", 1)] = struct{}{}
	}

	for _, fh := range all {
		if _, ok := cleaned[fh.Path]; !ok {
			want = append(want, fh)
		}
	}

	for _, wantHash := range want {
		path := filepath.Join(dest, wantHash.Path)

		if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
			return fmt.Errorf("mkdir %s: %w", path, err)
		}

		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("create file: %w", err)
		}

		if err := w.store.WriteTo(ctx, blob.CASKeyPrefix+"/"+wantHash.Hash, f); err != nil {
			return fmt.Errorf("write file (%s/%s): %w", wantHash.Path, wantHash.Hash, err)
		}
	}

	return nil
}
