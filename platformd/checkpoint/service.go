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

package checkpoint

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/uuid"
	"github.com/spacechunks/explorer/internal/image"
	"github.com/spacechunks/explorer/platformd/cri"
	"github.com/spacechunks/explorer/platformd/workload"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

const Namespace = "checkpoint"

type Service interface {
	CreateCheckpoint(ctx context.Context, baseRef name.Reference) (string, error)
	CheckpointStatus(checkpointID string) *Status
}

type svc struct {
	logger      *slog.Logger
	criService  cri.Service
	imgService  image.Service
	cfg         Config
	statusStore statusStore
}

func NewService(logger *slog.Logger, cfg Config, criService cri.Service) Service {
	return &svc{
		logger:      logger,
		criService:  criService,
		imgService:  image.NewService(logger, cfg.RegistryUser, cfg.RegistryPass, "/tmp"),
		cfg:         cfg,
		statusStore: newStore(),
	}
}

func (s *svc) CreateCheckpoint(ctx context.Context, baseRef name.Reference) (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}

	go func() {
		if err := s.checkpoint(ctx, id.String(), baseRef); err != nil {
			s.logger.ErrorContext(ctx, "error creating checkpoint", "err", err, "checkpoint_id", id.String())
		}
	}()

	return "", nil
}

func (s *svc) CheckpointStatus(checkpointID string) *Status {
	return s.statusStore.Get(checkpointID)
}

// CollectGarbage removes all checkpoint tarballs and pods of checkpoint jobs
// that have failed or completed successfully.
func (s *svc) CollectGarbage(ctx context.Context) error {
	var keep map[string]bool
	for id, status := range s.statusStore.View() {
		if status.State == StateRunning {
			keep[id] = true
		}

		// remove all statuses that are have failed or completed, so they
		// don't pile up in memory.
		//
		// we keep them for a certain amount of time, so callers of
		// the checkpoint api are able to retrieve the status after
		// checkpoint completion (or failure). removing them instantly
		// could lead to callers pulling the status endpoint to not
		// see the updated status.
		if time.Now().Sub(status.CompletedAt) >= s.cfg.StatusRetentionDuration {
			s.statusStore.Del(id)
		}
	}

	// cleanup all checkpoint tarballs in location dir

	for id := range keep {
		files, err := os.ReadDir(s.cfg.CheckpointLocationDir)
		if err != nil {
			return fmt.Errorf("read dir: %w", err)
		}

		for _, f := range files {
			if f.IsDir() {
				continue
			}

			if _, ok := keep[f.Name()]; ok {
				continue
			}

			if err := os.Remove(s.cfg.CheckpointLocationDir + "/" + f.Name()); err != nil {
				// if creating checkpoint failed before file could be written
				// to disk, this can happen. just ignore it.
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return fmt.Errorf("remove file: %w", err)
			}
			s.logger.InfoContext(
				ctx,
				"cleaned up image file",
				"path", s.cfg.CheckpointLocationDir+"/"+id,
				"checkpoint_id", id,
			)
		}
	}

	// clean up all zombie checkpoint pods

	resp, err := s.criService.ListPodSandbox(ctx, &runtimev1.ListPodSandboxRequest{
		Filter: &runtimev1.PodSandboxFilter{
			LabelSelector: map[string]string{
				workload.LabelWorkloadType: "checkpoint",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("list pods: %w", err)
	}

	for _, pod := range resp.Items {
		if _, ok := keep[pod.Metadata.Uid]; ok {
			continue
		}
		// FIXME: stop container of pod first then call stop sandbox.
		//        calling stop sandbox should also remove the stopped
		//        container.
		if _, err := s.criService.StopPodSandbox(ctx, &runtimev1.StopPodSandboxRequest{
			PodSandboxId: pod.Id,
		}); err != nil {
			return fmt.Errorf("stop pod: %w", err)
		}

		if _, err := s.criService.RemovePodSandbox(ctx, &runtimev1.RemovePodSandboxRequest{
			PodSandboxId: pod.Id,
		}); err != nil {
			return fmt.Errorf("remove pod: %w", err)
		}
	}

	return nil
}

func (s *svc) checkpoint(ctx context.Context, id string, baseRef name.Reference) (ret error) {
	s.logger.InfoContext(ctx, "creating checkpoint", "checkpoint", id, "baseRef", baseRef.String())

	s.statusStore.Update(id, Status{
		State: StateRunning,
	})

	state := StateCompleted

	defer func() {
		var msg string
		if ret != nil {
			msg = ret.Error()
		}
		s.statusStore.Update(id, Status{
			State:       state,
			Message:     msg,
			CompletedAt: time.Now(),
		})
	}()

	if _, err := s.criService.EnsureImage(ctx, baseRef.String()); err != nil {
		state = StatePullBaseImageFailed
		return fmt.Errorf("pull base image: %w", err)
	}

	ctrID, err := s.waitContainerReady(ctx, id, baseRef.String())
	if err != nil {
		state = StateContainerWaitReadyFailed
		return fmt.Errorf("wait ctr ready: %w", err)
	}

	s.logger.InfoContext(
		ctx,
		"done waiting for regex",
		"checkpoint_id", id,
		"regex", paperServerReadyRegex.String(),
	)

	location := fmt.Sprintf("%s/%s", s.cfg.CheckpointLocationDir, id)

	if _, err := s.criService.CheckpointContainer(ctx, &runtimev1.CheckpointContainerRequest{
		ContainerId: ctrID,
		Location:    location,
		Timeout:     s.cfg.CheckpointTimeoutSeconds,
	}); err != nil {
		state = StateContainerCheckpointFailed
		return fmt.Errorf("checkpoint container: %w", err)
	}

	s.logger.InfoContext(ctx, "checkpoint created", "checkpoint_id", id)

	img, err := image.FromCheckpoint(location, runtime.GOARCH, "/bin/sh", time.Now())
	if err != nil {
		state = StatePushCheckpointFailed
		return fmt.Errorf("create image: %w", err)
	}

	if err := s.imgService.Push(ctx, img, baseRef.Context().Tag("checkpoint").String()); err != nil {
		state = StatePushCheckpointFailed
		return fmt.Errorf("push image: %w", err)
	}

	return nil
}

func (s *svc) waitContainerReady(ctx context.Context, id string, baseImgURL string) (string, error) {
	ctrID, attachURL, err := s.runAndAttachContainer(ctx, id, baseImgURL)
	if err != nil {
		return "", fmt.Errorf("run and attach container: %w", err)
	}

	exec, err := spdyExecutor(attachURL)
	if err != nil {
		return "", fmt.Errorf("spdy executor: %w", err)
	}

	r := newLogReader(exec)

	if err := r.WaitForRegex(ctx, paperServerReadyRegex); err != nil {
		return "", fmt.Errorf("reading logs: %w", err)
	}

	return ctrID, nil
}

func (s *svc) runAndAttachContainer(ctx context.Context, id string, baseImgURL string) (string, string, error) {
	podCfg := &runtimev1.PodSandboxConfig{
		Metadata: &runtimev1.PodSandboxMetadata{
			Name:      id,
			Uid:       id,
			Namespace: Namespace,
		},
		Hostname:     id,
		LogDirectory: cri.PodLogDir,
		DnsConfig: &runtimev1.DNSConfig{
			Servers:  []string{"10.0.0.53"}, // TODO: make configurable
			Options:  []string{"edns0", "trust-ad"},
			Searches: []string{"."},
		},
		Labels: map[string]string{
			workload.LabelWorkloadID:   id,
			workload.LabelWorkloadType: "checkpoint",
		},
		Linux: &runtimev1.LinuxPodSandboxConfig{
			Resources: &runtimev1.LinuxContainerResources{
				CpuPeriod:          s.cfg.CPUPeriod,
				CpuQuota:           s.cfg.CPUQuota,
				MemoryLimitInBytes: s.cfg.MemoryLimitInBytes,
			},
		},
	}

	runPodResp, err := s.criService.RunPodSandbox(ctx, &runtimev1.RunPodSandboxRequest{
		Config: podCfg,
	})
	if err != nil {
		return "", "", fmt.Errorf("create pod: %w", err)
	}

	ctrID, err := s.criService.RunContainer(ctx, &runtimev1.CreateContainerRequest{
		PodSandboxId: runPodResp.PodSandboxId,
		Config: &runtimev1.ContainerConfig{
			Metadata: &runtimev1.ContainerMetadata{
				Name: "payload",
			},
			Image: &runtimev1.ImageSpec{
				UserSpecifiedImage: baseImgURL,
				Image:              baseImgURL,
			},
			Labels: map[string]string{
				workload.LabelWorkloadID:   id,
				workload.LabelWorkloadType: "checkpoint",
			},
			LogPath: fmt.Sprintf("%s_%s", Namespace, "payload"),
		},
		SandboxConfig: podCfg,
	})
	if err != nil {
		return "", "", fmt.Errorf("run container: %w", err)
	}

	attachResp, err := s.criService.Attach(ctx, &runtimev1.AttachRequest{
		ContainerId: ctrID,
		Stdout:      true,
	})
	if err != nil {
		return "", "", fmt.Errorf("attach container: %w", err)
	}

	return ctrID, attachResp.Url, nil
}
