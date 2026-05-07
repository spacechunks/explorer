package workload

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/resource"
	"github.com/spacechunks/explorer/platformd/cri"
	"github.com/spacechunks/explorer/platformd/status"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type Service interface {
	RunWorkload(ctx context.Context, w Workload, attempt uint) error
	RemoveWorkload(ctx context.Context, id string) error
	GetWorkloadHealth(ctx context.Context, id string) (status.WorkloadHealthStatus, error)
	WorkloadMetadata(ctx context.Context, id string) (Metadata, error)
}

type svc struct {
	logger       *slog.Logger
	criService   cri.Service
	registryAuth cri.RegistryAuth
	cfg          Config
}

func NewService(
	logger *slog.Logger,
	cfg Config,
	criService cri.Service,
	registryAuth cri.RegistryAuth,
) Service {
	return &svc{
		logger:       logger.With("component", "workload-service"),
		criService:   criService,
		registryAuth: registryAuth,
		cfg:          cfg,
	}
}

// RunWorkload calls the CRI to create a new pod based on the passed workload.
func (s *svc) RunWorkload(ctx context.Context, w Workload, attempt uint) error {
	logger := s.logger.With("workload_id", w.ID, "pod_name", w.Name, "namespace", w.Namespace)

	data, err := protojson.Marshal(w.Instance)
	if err != nil {
		return fmt.Errorf("marshalling instance: %w", err)
	}

	sboxCfg := &runtimev1.PodSandboxConfig{
		Metadata: &runtimev1.PodSandboxMetadata{
			Name:      w.Name,
			Uid:       w.ID,
			Namespace: w.Namespace,
			Attempt:   uint32(attempt),
		},
		Annotations: map[string]string{
			AnnotationInstance: string(data),
		},
		Hostname:     w.Hostname, // TODO: explore if we can use the id as the hostname
		LogDirectory: cri.PodLogDir,
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
	}²

	pulled, err := s.criService.EnsureImage(ctx, w.BaseImage, s.registryAuth)
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

	sboxResp, err := s.criService.RunPodSandbox(ctx, &runtimev1.RunPodSandboxRequest{
		Config: sboxCfg,
	})
	if err != nil {
		return fmt.Errorf("create pod: %w", err)
	}

	if _, err := s.criService.EnsureImage(ctx, w.CheckpointImage, s.registryAuth); err != nil {
		return fmt.Errorf("pull checkpoint image if not present: %w", err)
	}

	if _, err := s.criService.EnsureImage(ctx, s.cfg.ServerMonImage, cri.Unauthenticated); err != nil {
		return fmt.Errorf("pull servermon image if not present: %w", err)
	}

	logger = logger.With("pod_id", sboxResp.PodSandboxId)
	logger.InfoContext(ctx, "started pod sandbox")

	mcServerReq := &runtimev1.CreateContainerRequest{
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
			LogPath: fmt.Sprintf("%s_%s_%s", w.Namespace, w.ID, w.Name),
		},
		SandboxConfig: sboxCfg,
	}

	mcCtrID, err := s.criService.RunContainer(ctx, mcServerReq)
	if err != nil {
		return fmt.Errorf("run mc server container: %w", err)
	}

	logger.InfoContext(ctx, "started mc server container", "container_id", mcCtrID)

	serverMonReq := &runtimev1.CreateContainerRequest{
		PodSandboxId: sboxResp.PodSandboxId,
		Config: &runtimev1.ContainerConfig{
			Metadata: &runtimev1.ContainerMetadata{
				Name: "servermon",
			},
			Labels: map[string]string{
				LabelWorkloadID: w.ID,
			},
			Image: &runtimev1.ImageSpec{
				UserSpecifiedImage: s.cfg.ServerMonImage,
				Image:              s.cfg.ServerMonImage,
			},
			LogPath: fmt.Sprintf("%s_%s_%s", w.Namespace, w.ID, "servermon"),
			Mounts: []*runtimev1.Mount{
				{
					HostPath:      s.cfg.PlatformdListenSockURL.Path,
					ContainerPath: s.cfg.PlatformdListenSockURL.Path,
				},
			},
			Linux: &runtimev1.LinuxContainerConfig{
				SecurityContext: &runtimev1.LinuxContainerSecurityContext{
					RunAsUser: &runtimev1.Int64Value{
						Value: int64(s.cfg.PlatformdSocketUID),
					},
					RunAsGroup: &runtimev1.Int64Value{
						Value: int64(s.cfg.PlatformdSocketGID),
					},
				},
			},
			Envs: []*runtimev1.KeyValue{
				{
					Key:   "PLATFORMD_WORKLOAD_ID",
					Value: w.ID,
				},
				{
					Key:   "SERVERMON_MC_SERVER_MANAGEMENT_API_TOKEN",
					Value: s.cfg.MCManagementAPIToken,
				},
				{
					Key:   "SERVERMON_PLATFORMD_LISTEN_SOCK",
					Value: s.cfg.PlatformdListenSockURL.String(),
				},
			},
		},
		SandboxConfig: sboxCfg,
	}

	serverMonCtrID, err := s.criService.RunContainer(ctx, serverMonReq)
	if err != nil {
		return fmt.Errorf("run servermon container: %w", err)
	}

	logger.InfoContext(ctx, "started servermon container", "container_id", serverMonCtrID)

	return nil
}

func (s *svc) RemoveWorkload(ctx context.Context, id string) error {
	s.logger.InfoContext(ctx, "removing workload", "workload_id", id)
	listResp, err := s.criService.ListPodSandbox(ctx, &runtimev1.ListPodSandboxRequest{
		Filter: &runtimev1.PodSandboxFilter{
			LabelSelector: map[string]string{
				LabelWorkloadID: id,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("list pod sandbox: %w", err)
	}

	if len(listResp.Items) == 0 {
		s.logger.InfoContext(ctx, "skip removing workload, it's not found")
		return nil
	}

	podID := listResp.Items[0].Id

	// FIXME: stop container of pod first then call stop sandbox.
	if _, err := s.criService.StopPodSandbox(ctx, &runtimev1.StopPodSandboxRequest{
		PodSandboxId: podID,
	}); err != nil {
		return fmt.Errorf("stop pod sandbox: %w", err)
	}

	if _, err := s.criService.RemovePodSandbox(ctx, &runtimev1.RemovePodSandboxRequest{
		PodSandboxId: podID,
	}); err != nil {
		return fmt.Errorf("remove pod sandbox: %w", err)
	}

	resp, err := s.criService.ListContainers(ctx, &runtimev1.ListContainersRequest{
		Filter: &runtimev1.ContainerFilter{
			PodSandboxId: podID,
		},
	})
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}

	for _, c := range resp.Containers {
		if _, err := s.criService.RemoveContainer(ctx, &runtimev1.RemoveContainerRequest{
			ContainerId: c.Id,
		}); err != nil {
			s.logger.InfoContext(ctx,
				"removing container failed",
				"container_id", c.Id,
				"workload_id", id,
				"err", err,
			)
		}
	}

	return nil
}

// GetWorkloadHealth checks whether a container can be found for the given workload.
// if it cannot be found, or the status is CREATED, EXITED or UNKNOWN, the workload
// is considered unhealthy.
func (s *svc) GetWorkloadHealth(ctx context.Context, id string) (status.WorkloadHealthStatus, error) {
	resp, err := s.criService.ListContainers(ctx, &runtimev1.ListContainersRequest{
		Filter: &runtimev1.ContainerFilter{
			LabelSelector: map[string]string{
				LabelWorkloadID: id,
			},
		},
	})
	if err != nil {
		return status.WorkloadHealthStatusUnhealthy, fmt.Errorf("list containers: %w", err)
	}

	if len(resp.GetContainers()) == 0 {
		return status.WorkloadHealthStatusUnhealthy, nil
	}

	for _, c := range resp.Containers {
		switch c.State {
		case runtimev1.ContainerState_CONTAINER_RUNNING:
			return status.WorkloadHealthStatusHealthy, nil
		case runtimev1.ContainerState_CONTAINER_EXITED,
			runtimev1.ContainerState_CONTAINER_CREATED,
			runtimev1.ContainerState_CONTAINER_UNKNOWN:
			s.logger.InfoContext(
				ctx,
				"workload unhealthy due to container state",
				"state", c.State,
				"container_name", c.Metadata.Name,
				"container_id", c.Id,
				"workload_id", id,
			)
			return status.WorkloadHealthStatusUnhealthy, nil
		}
	}

	return status.WorkloadHealthStatusHealthy, nil
}

func (s *svc) WorkloadMetadata(ctx context.Context, id string) (Metadata, error) {
	listResp, err := s.criService.ListPodSandbox(ctx, &runtimev1.ListPodSandboxRequest{
		Filter: &runtimev1.PodSandboxFilter{
			LabelSelector: map[string]string{
				LabelWorkloadID: id,
			},
		},
	})
	if err != nil {
		return Metadata{}, fmt.Errorf("list pod sandbox: %w", err)
	}

	if len(listResp.Items) == 0 {
		return Metadata{}, grpcstatus.Error(codes.NotFound, "workload not found")
	}

	insData := listResp.Items[0].Annotations[AnnotationInstance]
	if insData == "" {
		return Metadata{}, grpcstatus.Error(codes.Internal, "invalid instance data")
	}

	instance := &instancev1alpha1.Instance{}
	if err := protojson.Unmarshal([]byte(insData), instance); err != nil {
		return Metadata{}, fmt.Errorf("unmarshal instance data: %w", err)
	}

	return Metadata{
		ID: instance.GetId(),
		Chunk: resource.Chunk{
			ID:          instance.Chunk.Id,
			Name:        instance.Chunk.Name,
			Description: instance.Chunk.Description,
			Tags:        instance.Chunk.Tags,
			CreatedAt:   instance.Chunk.CreatedAt.AsTime(),
			UpdatedAt:   instance.Chunk.CreatedAt.AsTime(),
		},
		FlavorVersion: chunk.FlavorVersionToDomain(instance.FlavorVersion),
		OrderedBy:     instance.OrderedBy,
	}, nil
}
