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
	"log/slog"
	"os"
	"path/filepath"

	"github.com/riverqueue/river"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/spacechunks/explorer/controlplane/serverconfig"
	"github.com/spacechunks/explorer/internal/file"
	"github.com/spacechunks/explorer/internal/image"
	"github.com/spacechunks/explorer/internal/resource"
	"go.opentelemetry.io/otel/trace"
)

type CreateImageWorkerConfig struct {
	ImagePlatform string
}

type CreateImageWorker struct {
	logger *slog.Logger
	river.WorkerDefaults[job.CreateImage]
	repo       chunk.Repository
	store      blob.S3Store
	imgService image.Service
	jobClient  job.Client
	cfg        CreateImageWorkerConfig
}

func NewCreateImageWorker(
	logger *slog.Logger,
	repo chunk.Repository,
	imgSvc image.Service,
	jobClient job.Client,
	store blob.S3Store,
	cfg CreateImageWorkerConfig,
) *CreateImageWorker {
	return &CreateImageWorker{
		logger:     logger,
		repo:       repo,
		store:      store,
		imgService: imgSvc,
		jobClient:  jobClient,
		cfg:        cfg,
	}
}

func (w *CreateImageWorker) Work(ctx context.Context, riverJob *river.Job[job.CreateImage]) (ret error) {
	span := trace.SpanFromContext(ctx)
	span.AddLink(trace.Link{
		SpanContext: riverJob.Args.SpanContext.OTel(),
	})

	defer func() {
		if ret == nil {
			return
		}

		w.logger.Error(
			"job failed",
			"chunk_id", riverJob.Args.ChunkID,
			"chunk_name", riverJob.Args.ChunkName,
			"flavor_id", riverJob.Args.FlavorID,
			"flavor_name", riverJob.Args.FlavorName,
			"err", ret,
		)

		// we only want to update the job to failed
		// once we exhausted all attempts.
		if riverJob.Attempt < riverJob.MaxAttempts {
			return
		}

		if err := w.repo.UpdateFlavorVersionBuildStatus(
			ctx,
			riverJob.Args.FlavorVersionID,
			resource.FlavorVersionBuildStatusBuildImageFailed,
		); err != nil {
			w.logger.ErrorContext(ctx, "failed to update flavor version build status", "err", err)
		}
	}()

	if err := riverJob.Args.Validate(); err != nil {
		return fmt.Errorf("validate args: %w", err)
	}

	baseImg, err := w.imgService.Pull(ctx, riverJob.Args.BaseImage, w.cfg.ImagePlatform)
	if err != nil {
		return fmt.Errorf("pull image: %w", err)
	}

	version, err := w.repo.FlavorVersionByID(ctx, riverJob.Args.FlavorVersionID)
	if err != nil {
		return fmt.Errorf("flavor version: %w", err)
	}

	if !version.FilesUploaded {
		return fmt.Errorf("files of flavor version %s have not been verified", version.ID)
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

	if err := os.MkdirAll(rootDir, os.ModePerm); err != nil {
		return fmt.Errorf("create root dir: %w", err)
	}

	if err := w.downloadFiles(ctx, serverRootDir, version.FileHashes); err != nil {
		return fmt.Errorf("download files: %w", err)
	}

	rt, err := os.OpenRoot(serverRootDir)
	if err != nil {
		return fmt.Errorf("open root: %w", err)
	}

	if err := serverconfig.SanitizeConfigs(rt); err != nil {
		return fmt.Errorf("sanitize configs: %w", err)
	}

	// it is VERY important we specify the parent of the server root directory,
	// because only paths starting INSIDE the passed directory are preserved.
	// so, in our case we specify files/opt/paper to keep the /opt/paper prefix.
	img, err := image.AppendLayer(baseImg, filesDir)
	if err != nil {
		return fmt.Errorf("append layer: %w", err)
	}

	ref := fmt.Sprintf("%s/%s:base", riverJob.Args.OCIRegistry, riverJob.Args.FlavorVersionID)

	if err := w.imgService.Push(ctx, img, ref); err != nil {
		return fmt.Errorf("push image: %w", err)
	}

	if err := w.jobClient.InsertJob(
		ctx,
		riverJob.Args.FlavorVersionID,
		string(resource.FlavorVersionBuildStatusBuildCheckpoint),
		job.CreateCheckpoint{
			FlavorVersionID: riverJob.Args.FlavorVersionID,
			BaseImageURL:    ref,
			SpanContext:     riverJob.Args.SpanContext,
			ChunkID:         riverJob.Args.ChunkID,
			ChunkName:       riverJob.Args.ChunkName,
			FlavorID:        riverJob.Args.FlavorID,
			FlavorName:      riverJob.Args.FlavorName,
		}); err != nil {
		return fmt.Errorf("insert create checkpoint job: %w", err)
	}

	return nil
}

func (w *CreateImageWorker) downloadFiles(ctx context.Context, dest string, files []file.Hash) error {
	for _, fh := range files {
		path := filepath.Join(dest, fh.Path)

		if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
			return fmt.Errorf("mkdir %s: %w", path, err)
		}

		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("create file: %w", err)
		}

		if err := w.store.WriteTo(ctx, blob.CASKeyPrefix+"/"+fh.Hash, f); err != nil {
			f.Close()
			return fmt.Errorf("write file (%s/%s): %w", fh.Path, fh.Hash, err)
		}

		f.Close()
	}

	return nil
}
