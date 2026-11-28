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
	"github.com/spacechunks/explorer/controlplane/instance"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/spacechunks/explorer/controlplane/serverapi"
	"github.com/spacechunks/explorer/internal/resource"
	"go.opentelemetry.io/otel/trace"
)

type CreateInstanceWorkerConfig struct {
	Timeout             time.Duration
	StatusCheckInterval time.Duration
	RegistryEndpoint    string
}

type CreateInstanceWorker struct {
	river.WorkerDefaults[job.CreateInstance]

	logger       *slog.Logger
	insRepo      instance.Repository
	serverClient serverapi.Client
	cfg          CreateInstanceWorkerConfig
}

func NewCreateInstanceWorker(
	logger *slog.Logger,
	insRepo instance.Repository,
	serverClient serverapi.Client,
	cfg CreateInstanceWorkerConfig,
) *CreateInstanceWorker {
	return &CreateInstanceWorker{
		logger:       logger,
		insRepo:      insRepo,
		serverClient: serverClient,
		cfg:          cfg,
	}
}

func (w *CreateInstanceWorker) Work(ctx context.Context, riverJob *river.Job[job.CreateInstance]) (ret error) {
	span := trace.SpanFromContext(ctx)
	span.AddLink(trace.Link{
		SpanContext: riverJob.Args.SpanContext.OTel(),
	})

	defer func() {
		if ret == nil {
			return
		}

		w.logger.ErrorContext(
			ctx,
			"job failed",
			"instance_id", riverJob.Args.InstanceID,
			"err", ret,
		)
	}()

	if err := riverJob.Args.Validate(); err != nil {
		return fmt.Errorf("validate args: %w", err)
	}

	ins, err := w.insRepo.GetInstanceByID(ctx, riverJob.Args.InstanceID)
	if err != nil {
		return fmt.Errorf("ins by id: %w", err)
	}

	image := fmt.Sprintf("%s/%s:base", w.cfg.RegistryEndpoint, ins.FlavorVersion.ID)

	serverID, err := w.serverClient.CreateServer(ctx, ins.ID, image)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	ticker := time.NewTicker(w.cfg.StatusCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			status, err := w.serverClient.GetServer(ctx, serverID)
			if err != nil {
				w.logger.ErrorContext(ctx, "get server status failed", "server_id", serverID, "err", err)
				continue
			}

			switch status.State {
			case serverapi.StateRunning:
				var port uint16
				if status.Network != nil {
					port = status.Network.Port
				}

				if err := w.insRepo.ApplyStatusReports(ctx, []resource.InstanceStatusReport{
					{
						InstanceID: riverJob.Args.InstanceID,
						State:      resource.InstanceStateRunning,
						Port:       port,
					},
				}); err != nil {
					return fmt.Errorf("apply status report: %w", err)
				}

				w.logger.InfoContext(
					ctx,
					"instance running",
					"instance_id", riverJob.Args.InstanceID,
					"server_id", serverID,
				)

				return nil
			case serverapi.StateError:
				if err := w.serverClient.DeleteServer(ctx, serverID); err != nil {
					w.logger.ErrorContext(ctx, "failed to delete errored server", "server_id", serverID, "err", err)
				}

				if err := w.insRepo.ApplyStatusReports(ctx, []resource.InstanceStatusReport{
					{
						InstanceID: riverJob.Args.InstanceID,
						State:      resource.InstanceStateDeleted,
					},
				}); err != nil {
					w.logger.ErrorContext(
						ctx,
						"failed to update instance state",
						"instance_id", riverJob.Args.InstanceID,
						"err", err,
					)
				}

				return river.JobCancel(fmt.Errorf("server %s entered error state", serverID))
			default:
				// server still transitioning (pending/creating/updating/deleting), keep polling
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (w *CreateInstanceWorker) Timeout(*river.Job[job.CreateInstance]) time.Duration {
	return w.cfg.Timeout
}
