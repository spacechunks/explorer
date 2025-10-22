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
	"github.com/spacechunks/explorer/internal/ptr"
	"github.com/spacechunks/explorer/platformd/cri"
	"github.com/spacechunks/explorer/platformd/status"
	"github.com/spacechunks/explorer/platformd/workload"
	"k8s.io/client-go/tools/remotecommand"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

const Namespace = "checkpoint"

type RemoteCMDExecutorFactory func(url string) (remotecommand.Executor, error)

type Service interface {
	CreateCheckpoint(ctx context.Context, baseRef name.Reference) (string, error)
	CheckpointStatus(checkpointID string) *status.Status
}

type ServiceImpl struct {
	logger      *slog.Logger
	criService  cri.Service
	imgService  image.Service
	cfg         Config
	statusStore status.Store
	portAlloc   *workload.PortAllocator

	// this factory function allows us to inject a mock executor for testing.
	newRemoteCMDExecutor RemoteCMDExecutorFactory
}

func NewService(
	logger *slog.Logger,
	cfg Config,
	criService cri.Service,
	imgService image.Service,
	statusStore status.Store,
	newRemoteCMDExecutor RemoteCMDExecutorFactory,
	portAlloc *workload.PortAllocator,

) *ServiceImpl {
	return &ServiceImpl{
		logger:               logger,
		criService:           criService,
		imgService:           imgService,
		cfg:                  cfg,
		statusStore:          statusStore,
		portAlloc:            portAlloc,
		newRemoteCMDExecutor: newRemoteCMDExecutor,
	}
}

func (s *ServiceImpl) CreateCheckpoint(ctx context.Context, baseRef name.Reference) (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}

	go func() {
		if err := s.checkpoint(ctx, id.String(), baseRef); err != nil {
			s.logger.ErrorContext(ctx, "error creating checkpoint", "err", err, "checkpoint_id", id.String())
		}
	}()

	return id.String(), nil
}

func (s *ServiceImpl) CheckpointStatus(checkpointID string) *status.Status {
	return s.statusStore.Get(checkpointID)
}

// CollectGarbage removes all checkpoint tarballs and pods of checkpoint jobs
// that have failed or completed successfully.
func (s *ServiceImpl) CollectGarbage(ctx context.Context) error {
	keep := make(map[string]bool)
	for id, st := range s.statusStore.View() {
		if st.CheckpointStatus == nil {
			continue
		}

		if st.CheckpointStatus.State == status.CheckpointStateRunning {
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
		if st.CheckpointStatus.CompletedAt != nil && time.Now().After(st.CheckpointStatus.CompletedAt.Add(s.cfg.StatusRetentionPeriod)) {
			// FIXME: we should probably make sure we only delete the entry
			//        if the pod is also gone. because once we remove from
			//        the store, the cni will not have port information
			//        which is needed to remove bpf map entries
			s.statusStore.Del(id)
			s.portAlloc.Free(st.CheckpointStatus.Port)
		}
	}

	// cleanup all checkpoint tarballs in location dir

	files, err := os.ReadDir(s.cfg.CheckpointFileDir)
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

		if err := os.Remove(s.cfg.CheckpointFileDir + "/" + f.Name()); err != nil {
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
			"path", s.cfg.CheckpointFileDir+"/"+f.Name(),
			"checkpoint_id", f.Name(),
		)
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

		s.logger.InfoContext(ctx, "cleaned up pod", "checkpoint_id", pod.Metadata.Uid, "pod_id", pod.Id)
	}

	return nil
}

func (s *ServiceImpl) checkpoint(ctx context.Context, id string, baseRef name.Reference) (ret error) {
	s.logger.InfoContext(ctx, "creating checkpoint", "checkpoint", id, "baseRef", baseRef.String())

	s.statusStore.Update(id, status.Status{
		CheckpointStatus: &status.CheckpointStatus{
			State: status.CheckpointStateRunning,
		},
	})

	state := status.CheckpointStateCompleted

	defer func() {
		var msg string
		if ret != nil {
			msg = ret.Error()
		}

		s.statusStore.Update(id, status.Status{
			CheckpointStatus: &status.CheckpointStatus{
				State:       state,
				Message:     msg,
				CompletedAt: ptr.Pointer(time.Now()),
			},
		})
	}()

	if _, err := s.criService.EnsureImage(ctx, baseRef.String(), cri.RegistryAuth{
		Username: s.cfg.RegistryUser,
		Password: s.cfg.RegistryPass,
	}); err != nil {
		state = status.CheckpointStatePullBaseImageFailed
		return fmt.Errorf("pull base image: %w", err)
	}

	s.logger.InfoContext(
		ctx,
		"waiting for regex",
		"checkpoint_id", id,
		"regex", paperServerReadyRegex.String(),
	)

	ctrID, err := s.waitContainerReady(ctx, id, baseRef.String(), s.cfg.ContainerReadyTimeout)
	if err != nil {
		state = status.CheckpointStateContainerWaitReadyFailed
		return fmt.Errorf("wait ctr ready: %w", err)
	}

	// TODO: check if checkpoint is already present and then only build image and push
	location := fmt.Sprintf("%s/%s", s.cfg.CheckpointFileDir, id)

	s.logger.InfoContext(ctx, "checkpointing container", "checkpoint_id", id, "container_id", ctrID)

	if _, err := s.criService.CheckpointContainer(ctx, &runtimev1.CheckpointContainerRequest{
		ContainerId: ctrID,
		Location:    location,
		Timeout:     s.cfg.CheckpointTimeoutSeconds,
	}); err != nil {
		state = status.CheckpointStateContainerCheckpointFailed
		return fmt.Errorf("checkpoint container: %w", err)
	}

	img, err := image.FromCheckpoint(location, runtime.GOARCH, "/bin/sh", time.Now())
	if err != nil {
		state = status.CheckpointStatePushCheckpointFailed
		return fmt.Errorf("create image: %w", err)
	}

	if err := s.imgService.Push(ctx, img, baseRef.Context().Tag("checkpoint").String()); err != nil {
		state = status.CheckpointStatePushCheckpointFailed
		return fmt.Errorf("push image: %w", err)
	}

	return nil
}

func (s *ServiceImpl) waitContainerReady(
	ctx context.Context,
	id string,
	baseImgURL string,
	timeout time.Duration,
) (string, error) {
	ctrID, attachURL, err := s.runAndAttachContainer(ctx, id, baseImgURL)
	if err != nil {
		return "", fmt.Errorf("run and attach container: %w", err)
	}

	exec, err := s.newRemoteCMDExecutor(attachURL)
	if err != nil {
		return "", fmt.Errorf("spdy executor: %w", err)
	}

	r := newLogReader(exec)

	// do not use ctx as parent for this context, because we only want to
	// cause WaitForRegex to stop executing if the timeout is reached and
	// not the other functions that have ctx being passed to them prior
	// to the waitContainerReady call.
	timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := r.WaitForRegex(timeoutCtx, paperServerReadyRegex); err != nil {
		return "", fmt.Errorf("reading logs: %w", err)
	}

	return ctrID, nil
}

func (s *ServiceImpl) runAndAttachContainer(ctx context.Context, id string, baseImgURL string) (string, string, error) {
	podCfg := s.podConfig(id)

	port, err := s.portAlloc.Allocate()
	if err != nil {
		return "", "", fmt.Errorf("allocate port: %w", err)
	}

	s.statusStore.Update(id, status.Status{
		CheckpointStatus: &status.CheckpointStatus{
			State: status.CheckpointStateRunning,
			Port:  port,
		},
	})

	runPodResp, err := s.criService.RunPodSandbox(ctx, &runtimev1.RunPodSandboxRequest{
		Config: podCfg,
	})
	if err != nil {
		return "", "", fmt.Errorf("create pod: %w", err)
	}

	ctrID, err := s.criService.RunContainer(ctx, &runtimev1.CreateContainerRequest{
		PodSandboxId:  runPodResp.PodSandboxId,
		Config:        s.ctrConfig(id, baseImgURL),
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

func (s *ServiceImpl) podConfig(id string) *runtimev1.PodSandboxConfig {
	return &runtimev1.PodSandboxConfig{
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
				MemoryLimitInBytes: s.cfg.MemoryLimitBytes,
			},
		},
	}
}

func (s *ServiceImpl) ctrConfig(checkID string, baseImgURL string) *runtimev1.ContainerConfig {
	return &runtimev1.ContainerConfig{
		Metadata: &runtimev1.ContainerMetadata{
			Name: "payload",
		},
		Image: &runtimev1.ImageSpec{
			UserSpecifiedImage: baseImgURL,
			Image:              baseImgURL,
		},
		Labels: map[string]string{
			workload.LabelWorkloadID:   checkID,
			workload.LabelWorkloadType: "checkpoint",
		},
		LogPath: fmt.Sprintf("%s_%s", Namespace, "payload"),
	}
}
