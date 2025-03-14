package workload

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

const PodLogDir = "/var/log/platformd/pods"

var (
	ErrWorkloadNotFound  = errors.New("workload not found")
	ErrContainerNotFound = errors.New("container not found")
)

type Service interface {
	RunWorkload(ctx context.Context, w Workload, attempt int) error
	RemoveWorkload(ctx context.Context, id string) error
	GetWorkloadHealth(ctx context.Context, id string) (HealthStatus, error)
	EnsureWorkload(ctx context.Context, w Workload, labelSelector map[string]string) error
}

type criService struct {
	logger    *slog.Logger
	rtClient  runtimev1.RuntimeServiceClient
	imgClient runtimev1.ImageServiceClient
}

func NewService(
	logger *slog.Logger,
	rtClient runtimev1.RuntimeServiceClient,
	imgClient runtimev1.ImageServiceClient,
) Service {
	return &criService{
		logger:    logger.With("component", "workload-service"),
		rtClient:  rtClient,
		imgClient: imgClient,
	}
}

// EnsureWorkload ensures that a pod is created if not present.
// if ListPodSandbox returns 0 items, a pod with the passed configuration is created.
// Currently, this function is designed for a single item returned by the label selector.
// If multiple items are returned the first one will be picked.
func (s *criService) EnsureWorkload(ctx context.Context, w Workload, labelSelector map[string]string) error {
	resp, err := s.rtClient.ListPodSandbox(ctx, &runtimev1.ListPodSandboxRequest{
		Filter: &runtimev1.PodSandboxFilter{
			LabelSelector: labelSelector,
		},
	})
	if err != nil {
		return fmt.Errorf("list pod sandbox: %w", err)
	}

	// TODO: what do we do if the pod found is in NOT_READY state

	if len(resp.Items) > 0 {
		return nil
	}

	s.logger.InfoContext(ctx,
		"no matching workload found, creating pod",
		"pod_name", w.Name,
		"namespace", w.Namespace,
		"label_selector", labelSelector,
	)

	if err := s.RunWorkload(ctx, w, 0); err != nil {
		return fmt.Errorf("create pod: %w", err)
	}
	return nil
}

// RunWorkload calls the CRI to create a new pod based on the passed workload.
func (s *criService) RunWorkload(ctx context.Context, w Workload, attempt int) error {
	logger := s.logger.With("workload_id", w.ID, "pod_name", w.Name, "namespace", w.Namespace)

	sboxCfg := &runtimev1.PodSandboxConfig{
		Metadata: &runtimev1.PodSandboxMetadata{
			Name:      w.Name,
			Uid:       w.ID,
			Namespace: w.Namespace,
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
			SecurityContext: &runtimev1.LinuxSandboxSecurityContext{
				NamespaceOptions: &runtimev1.NamespaceOption{
					Network: runtimev1.NamespaceMode(w.NetworkNamespaceMode),
				},
			},
		},
	}

	sboxResp, err := s.rtClient.RunPodSandbox(ctx, &runtimev1.RunPodSandboxRequest{
		Config: sboxCfg,
	})
	if err != nil {
		return fmt.Errorf("create pod: %w", err)
	}

	if err := s.pullImageIfNotPresent(ctx, logger, w.Image); err != nil {
		return fmt.Errorf("pull image if not present: %w", err)
	}

	logger = logger.With("pod_id", sboxResp.PodSandboxId)
	logger.InfoContext(ctx, "started pod sandbox")

	req := &runtimev1.CreateContainerRequest{
		PodSandboxId: sboxResp.PodSandboxId,
		Config: &runtimev1.ContainerConfig{
			Metadata: &runtimev1.ContainerMetadata{
				Name:    w.Name,
				Attempt: uint32(attempt),
			},
			Image: &runtimev1.ImageSpec{
				UserSpecifiedImage: w.Image,
				Image:              w.Image,
			},
			Labels:  w.Labels,
			LogPath: fmt.Sprintf("%s_%s", w.Namespace, w.Name),
			Args:    w.Args,
		},
		SandboxConfig: sboxCfg,
	}

	ctrResp, err := s.rtClient.CreateContainer(ctx, req)
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}

	if _, err := s.rtClient.StartContainer(ctx, &runtimev1.StartContainerRequest{
		ContainerId: ctrResp.ContainerId,
	}); err != nil {
		return fmt.Errorf("start container: %w", err)
	}

	logger.InfoContext(ctx, "started container", "container_id", ctrResp.ContainerId)
	return nil
}

func (s *criService) RemoveWorkload(ctx context.Context, id string) error {
	s.logger.InfoContext(ctx, "removing workload", "workload_id", id)
	// FIXME: stop container of pod first then call stop sandbox.
	//        calling stop sandbox should also remove the stopped
	//        container.
	if _, err := s.rtClient.StopPodSandbox(ctx, &runtimev1.StopPodSandboxRequest{
		PodSandboxId: id,
	}); err != nil {
		return fmt.Errorf("stop pod sandbox: %w", err)
	}

	return nil
}

// GetWorkloadHealth checks whether a container can be found for the given workload.
// if it cannot be found, or the status is CREATED, EXITED or UNKOWN, the workload
// is considered unhealthy.
func (s *criService) GetWorkloadHealth(ctx context.Context, id string) (HealthStatus, error) {
	resp, err := s.rtClient.ListContainers(ctx, &runtimev1.ListContainersRequest{
		Filter: &runtimev1.ContainerFilter{
			PodSandboxId: id,
		},
	})
	if err != nil {
		return Unhealthy, fmt.Errorf("list containers: %w", err)
	}

	if len(resp.GetContainers()) == 0 {
		return Unhealthy, nil
	}

	switch resp.GetContainers()[0].State {
	case runtimev1.ContainerState_CONTAINER_RUNNING:
		return Healthy, nil
	case runtimev1.ContainerState_CONTAINER_CREATED:
	case runtimev1.ContainerState_CONTAINER_UNKNOWN:
	case runtimev1.ContainerState_CONTAINER_EXITED:
		return Unhealthy, nil
	}

	return Healthy, nil
}

// pullImageIfNotPresent first calls ListImages then checks if the image is contained in the response.
// if this is not the case PullImage is being called. this function does not access the services logger,
// and instead uses a passed one, to preserve arguments which provide additional context to the image pull.
func (s *criService) pullImageIfNotPresent(ctx context.Context, logger *slog.Logger, imageURL string) error {
	listResp, err := s.imgClient.ListImages(ctx, &runtimev1.ListImagesRequest{})
	if err != nil {
		return fmt.Errorf("list images: %w", err)
	}

	var img *runtimev1.Image
	for _, tmp := range listResp.Images {
		if slices.Contains(tmp.RepoTags, imageURL) {
			img = tmp
			break
		}
	}

	if img != nil {
		return nil
	}

	logger = logger.With("image", imageURL)
	logger.InfoContext(ctx, "pulling image")

	if _, err := s.imgClient.PullImage(ctx, &runtimev1.PullImageRequest{
		Image: &runtimev1.ImageSpec{
			Image: imageURL,
		},
	}); err != nil {
		return fmt.Errorf("pull image: %w", err)
	}

	logger.InfoContext(ctx, "image pulled")
	return nil
}
