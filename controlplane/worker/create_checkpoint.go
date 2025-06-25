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
	"time"

	"github.com/riverqueue/river"
	checkpointv1alpha1 "github.com/spacechunks/explorer/api/platformd/checkpoint/v1alpha1"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/spacechunks/explorer/controlplane/node"
)

type CreateCheckpointClient func(host string) (checkpointv1alpha1.CheckpointServiceClient, error)

type CreateCheckpointWorker struct {
	river.WorkerDefaults[job.CreateCheckpoint]

	logger              *slog.Logger
	timeout             time.Duration
	statusCheckInterval time.Duration
	nodeRepo            node.Repository
	chunkRepo           chunk.Repository

	// this factory function allows us to inject a mock client for testing.
	createCheckpointClient CreateCheckpointClient
}

func NewCheckpointWorker(
	logger *slog.Logger,
	createCheckpointClient CreateCheckpointClient,
	timeout time.Duration,
	statusCheckInterval time.Duration,
	nodeRepo node.Repository,
	chunkRepo chunk.Repository,
) *CreateCheckpointWorker {
	return &CreateCheckpointWorker{
		logger:                 logger,
		createCheckpointClient: createCheckpointClient,
		timeout:                timeout,
		statusCheckInterval:    statusCheckInterval,
		nodeRepo:               nodeRepo,
		chunkRepo:              chunkRepo,
	}
}

func (w *CreateCheckpointWorker) Work(ctx context.Context, riverJob *river.Job[job.CreateCheckpoint]) (ret error) {
	defer func() {
		if ret == nil {
			return
		}

		// we only want to update the job to failed
		// once we exhausted all attempts.
		if riverJob.Attempt != riverJob.MaxAttempts {
			return
		}

		if err := w.chunkRepo.UpdateFlavorVersionBuildStatus(
			ctx,
			riverJob.Args.FlavorVersionID,
			chunk.BuildStatusBuildCheckpointFailed,
		); err != nil {
			w.logger.ErrorContext(ctx, "failed to update flavor version build status", "err", err)
		}
	}()

	if err := riverJob.Args.Validate(); err != nil {
		return fmt.Errorf("validate args: %w", err)
	}

	// TODO: check if checkpoint already exists in repo.
	//       it could happen that things take too long
	//       and the job times out, but in the background
	//       platformd still executes the checkpointing
	//       and pushes an image.

	// TODO: choose only nodes that are available for running checkpoint workloads
	n, err := w.nodeRepo.RandomNode(ctx)
	if err != nil {
		return fmt.Errorf("random node: %w", err)
	}

	c, err := w.createCheckpointClient(n.CheckpointAPIEndpoint.String())
	if err != nil {
		return fmt.Errorf("create checkpoint client: %w", err)
	}

	resp, err := c.CreateCheckpoint(ctx, &checkpointv1alpha1.CreateCheckpointRequest{
		BaseImageUrl: riverJob.Args.BaseImageURL,
	})
	if err != nil {
		return fmt.Errorf("create checkpoint: %w", err)
	}

	t := time.NewTicker(w.statusCheckInterval)

	for {
		select {
		case <-t.C:
			statusResp, err := c.CheckpointStatus(ctx, &checkpointv1alpha1.CheckpointStatusRequest{
				CheckpointId: resp.CheckpointId,
			})
			if err != nil {
				w.logger.ErrorContext(ctx, "checkpoint status error", "err", err)
				continue
			}

			if statusResp.Status.State == checkpointv1alpha1.CheckpointState_RUNNING {
				continue
			}

			if statusResp.Status.State == checkpointv1alpha1.CheckpointState_COMPLETED {
				w.logger.InfoContext(ctx, "checkpointing completed", "checkpoint_id", resp.CheckpointId)
				if err := w.chunkRepo.UpdateFlavorVersionBuildStatus(
					ctx,
					riverJob.Args.FlavorVersionID,
					chunk.BuildStatusCompleted,
				); err != nil {
					return fmt.Errorf("flavor version build status: %w", err)
				}
				return nil
			}

			w.logger.ErrorContext(
				ctx,
				"error occurred while checkpointing",
				"status", statusResp.Status.State,
				"message", statusResp.Status.Message,
			)

			return fmt.Errorf("checkpointing failed: %v", statusResp.Status.Message)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (w *CreateCheckpointWorker) Timeout(*river.Job[job.CreateCheckpoint]) time.Duration {
	return w.timeout
}
