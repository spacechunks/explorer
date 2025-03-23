package workload

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/spacechunks/explorer/platformd/cri"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

const PodLogDir = "/var/log/platformd/pods"

type Service interface {
	RunWorkload(ctx context.Context, w Workload, attempt uint) error
	RemoveWorkload(ctx context.Context, id string) error
	GetWorkloadHealth(ctx context.Context, id string) (HealthStatus, error)
}

type svc struct {
	logger     *slog.Logger
	criService cri.Service
}

func NewService(
	logger *slog.Logger,
	criService cri.Service,
) Service {
	return &svc{
		logger:     logger.With("component", "workload-service"),
		criService: criService,
	}
}

// RunWorkload calls the CRI to create a new pod based on the passed workload.
func (s *svc) RunWorkload(ctx context.Context, w Workload, attempt uint) error {
	logger := s.logger.With("workload_id", w.ID, "pod_name", w.Name, "namespace", w.Namespace)

	sboxCfg := &runtimev1.PodSandboxConfig{
		Metadata: &runtimev1.PodSandboxMetadata{
			Name:      w.Name,
			Uid:       w.ID,
			Namespace: w.Namespace,
			Attempt:   uint32(attempt),
		},
		Hostname:     w.Hostname, // TODO: explore if we can use the id as the hostname
		LogDirectory: PodLogDir,
		Labels:       w.Labels,
		DnsConfig: &runtimev1.DNSConfig{
			Servers:  []string{"10.0.0.53"}, // TODO: make configurable
			Options:  []string{"edns0", "trust-ad"},
			Searches: []string{"."},
		},
		Linux: &runtimev1.LinuxPodSandboxConfig{
			Resources: &runtimev1.LinuxContainerResources{
				CpuPeriod:          int64(w.CPUPeriod),
				CpuQuota:           int64(w.CPUQuota),
				MemoryLimitInBytes: int64(w.MemoryLimitBytes),
			},
		},
	}

	pulled, err := s.criService.EnsureImage(ctx, w.BaseImage)
	if err != nil {
		return fmt.Errorf("pull image if not present: %w", err)
	}

	// HACK START - there is currently a strange behavior in crio (maybe a bug)
	// that when freshly pulling the base image, restoring the checkpoint after
	// will fail. this can be fixed by restarting crio and then restoring. until
	// this has been further investigated, use this as a workaround.
	//
	// keep it behind env var guard, to make testing easier.
	if ok := os.Getenv("PLATFORMD_ENABLE_CRIO_RESTART"); ok == "true" && pulled {
		s.logger.InfoContext(ctx, "restarting crio")
		if out, err := exec.Command("systemctl", "restart", "crio").CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl restart crio: %w: %s", err, out)
		}
		time.Sleep(5 * time.Second)
	}
	// HACK END

	sboxResp, err := s.criService.GetRuntimeClient().RunPodSandbox(ctx, &runtimev1.RunPodSandboxRequest{
		Config: sboxCfg,
	})
	if err != nil {
		return fmt.Errorf("create pod: %w", err)
	}

	if _, err := s.criService.EnsureImage(ctx, w.CheckpointImage); err != nil {
		return fmt.Errorf("pull image if not present: %w", err)
	}

	logger = logger.With("pod_id", sboxResp.PodSandboxId)
	logger.InfoContext(ctx, "started pod sandbox")

	req := &runtimev1.CreateContainerRequest{
		PodSandboxId: sboxResp.PodSandboxId,
		Config: &runtimev1.ContainerConfig{
			Metadata: &runtimev1.ContainerMetadata{
				Name: w.Name,
			},
			Image: &runtimev1.ImageSpec{
				UserSpecifiedImage: w.CheckpointImage,
				Image:              w.CheckpointImage,
			},
			Labels:  w.Labels,
			LogPath: fmt.Sprintf("%s_%s", w.Namespace, w.Name),
		},
		SandboxConfig: sboxCfg,
	}

	ctrID, err := s.criService.RunContainer(ctx, req)
	if err != nil {
		return fmt.Errorf("run container: %w", err)
	}

	logger.InfoContext(ctx, "started container", "container_id", ctrID)
	return nil
}

func (s *svc) RemoveWorkload(ctx context.Context, id string) error {
	s.logger.InfoContext(ctx, "removing workload", "workload_id", id)
	// FIXME: stop container of pod first then call stop sandbox.
	//        calling stop sandbox should also remove the stopped
	//        container.
	if _, err := s.criService.GetRuntimeClient().StopPodSandbox(ctx, &runtimev1.StopPodSandboxRequest{
		PodSandboxId: id,
	}); err != nil {
		return fmt.Errorf("stop pod sandbox: %w", err)
	}

	return nil
}

// GetWorkloadHealth checks whether a container can be found for the given workload.
// if it cannot be found, or the status is CREATED, EXITED or UNKNOWN, the workload
// is considered unhealthy.
func (s *svc) GetWorkloadHealth(ctx context.Context, id string) (HealthStatus, error) {
	resp, err := s.criService.GetRuntimeClient().ListContainers(ctx, &runtimev1.ListContainersRequest{
		Filter: &runtimev1.ContainerFilter{
			LabelSelector: map[string]string{
				LabelWorkloadID: id,
			},
		},
	})
	if err != nil {
		return HealthStatusUnhealthy, fmt.Errorf("list containers: %w", err)
	}

	if len(resp.GetContainers()) == 0 {
		return HealthStatusUnhealthy, nil
	}

	switch resp.GetContainers()[0].State {
	case runtimev1.ContainerState_CONTAINER_RUNNING:
		return HealthStatusHealthy, nil
	case runtimev1.ContainerState_CONTAINER_CREATED:
	case runtimev1.ContainerState_CONTAINER_UNKNOWN:
	case runtimev1.ContainerState_CONTAINER_EXITED:
		return HealthStatusUnhealthy, nil
	}

	return HealthStatusHealthy, nil
}
