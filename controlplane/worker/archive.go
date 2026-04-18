package worker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/riverqueue/river"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/instance"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/spacechunks/explorer/controlplane/resource"
)

type ArchiveWorker struct {
	river.WorkerDefaults[job.Archive]

	logger      *slog.Logger
	chunkRepo   chunk.Repository
	insRepo     instance.Repository
	archiveRepo chunk.ArchiveRepository
}

func NewArchiveWorker(
	logger *slog.Logger,
	chunkRepo chunk.Repository,
	insRepo instance.Repository,
	archiveRepo chunk.ArchiveRepository,
) *ArchiveWorker {
	return &ArchiveWorker{
		logger:      logger,
		chunkRepo:   chunkRepo,
		insRepo:     insRepo,
		archiveRepo: archiveRepo,
	}
}

func (w *ArchiveWorker) Work(ctx context.Context, _ *river.Job[job.Archive]) error {
	flavorIDToChunkIDs, err := w.chunkRepo.AllDeletedFlavors(ctx)
	if err != nil {
		return fmt.Errorf("get all deleted flavors: %w", err)
	}

	for flavorID, chunkID := range flavorIDToChunkIDs {
		logger := w.logger.With("chunk_id", chunkID, "flavor_id", flavorID)

		if err := w.tryArchiveFlavor(ctx, logger, chunkID, flavorID); err != nil {
			logger.ErrorContext(
				ctx,
				"failed to archive flavor",
				"err", err,
			)
			continue
		}

		c, err := w.chunkRepo.GetChunkByID(ctx, chunkID)
		if err != nil {
			logger.ErrorContext(
				ctx,
				"failed to get chunk",
				"err", err,
			)
			continue
		}

		if len(c.Flavors) > 0 {
			logger.InfoContext(ctx, "chunk still has flavors")
			continue
		}

		if err := w.archiveRepo.ArchiveChunk(ctx, c); err != nil {
			logger.ErrorContext(
				ctx,
				"failed to archive chunk",
				"err", err,
			)
			continue
		}
	}

	return nil
}

func (w *ArchiveWorker) tryArchiveFlavor(
	ctx context.Context,
	logger *slog.Logger,
	chunkID string,
	flavorID string,
) error {
	f, err := w.chunkRepo.GetFlavorByID(ctx, flavorID)
	if err != nil {
		return fmt.Errorf("get flavor: %w", err)
	}

	for _, version := range f.Versions {
		if err := w.tryArchiveFlavorVersion(ctx, logger, flavorID, version); err != nil {
			logger.ErrorContext(
				ctx,
				"failed to archive flavor version",
				"flavor_id", f.ID,
				"flavor_version_id", version.ID,
				"err", err,
			)
			continue
		}
	}

	updated, err := w.chunkRepo.GetFlavorByID(ctx, flavorID)
	if err != nil {
		return fmt.Errorf("get flavor: %w", err)
	}

	if len(updated.Versions) > 0 {
		return nil
	}

	logger.InfoContext(ctx, "archiving flavor", "flavor_id", f.ID)

	if err := w.archiveRepo.ArchiveFlavor(ctx, chunkID, f); err != nil {
		return fmt.Errorf("archive flavor: %w", err)
	}

	return nil
}

func (w *ArchiveWorker) tryArchiveFlavorVersion(
	ctx context.Context,
	logger *slog.Logger,
	flavorID string,
	version resource.FlavorVersion,
) error {
	c, err := w.insRepo.CountInstancesByFlavorVersionID(ctx, version.ID)
	if err != nil {
		return fmt.Errorf("count instances: %w", err)
	}

	if c > 0 {
		logger.InfoContext(ctx, "there are still running instance for flavor version")
		return nil
	}

	if version.BuildStatus != resource.FlavorVersionBuildStatusBuildCheckpointFailed &&
		version.BuildStatus != resource.FlavorVersionBuildStatusBuildImageFailed &&
		version.BuildStatus != resource.FlavorVersionBuildStatusCompleted {
		logger.InfoContext(
			ctx,
			"flavor version is still building. waiting for build to fail or finish",
			"build_status", version.BuildStatus,
			"flavor_version_id", version.ID,
		)
		return nil
	}

	logger.InfoContext(ctx, "archiving flavor version", "flavor_version_id", version.ID)

	if err := w.archiveRepo.ArchiveFlavorVersion(ctx, flavorID, version); err != nil {
		return fmt.Errorf("archive flavor version: %w", err)
	}

	return nil
}
