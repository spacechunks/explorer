package cri

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

const (
	PodLogDir   = "/var/log/platformd/pods"
	LabelPodUID = "io.kubernetes.pod.uid"
)

type RunOptions struct {
	PodConfig       *runtimev1.PodSandboxConfig
	ContainerConfig *runtimev1.ContainerConfig
}

// Service provides access to the cri api. its main purpose is to
// provide QoL functions. [runtimev1.RuntimeServiceClient] is embedded
// to avoid needing a getter or having to implement wrapper functions.
type Service interface {
	runtimev1.RuntimeServiceClient

	// EnsurePod makes sure the pod with the configured uid is present.
	// If not, all necessary resources will be created.
	EnsurePod(ctx context.Context, opts RunOptions) error

	// RunContainer creates the container and starts it.
	// Returns the started containers ID.
	RunContainer(ctx context.Context, req *runtimev1.CreateContainerRequest) (string, error)

	// EnsureImage makes sure that the OCI image with the given url is present.
	// Returns true if pulling was necessary, false if not.
	EnsureImage(ctx context.Context, imageURL string) (bool, error)
}

type svc struct {
	runtimev1.RuntimeServiceClient

	logger    *slog.Logger
	imgClient runtimev1.ImageServiceClient
}

func NewService(
	logger *slog.Logger,
	rtClient runtimev1.RuntimeServiceClient,
	imgClient runtimev1.ImageServiceClient,
) Service {
	return &svc{
		RuntimeServiceClient: rtClient,
		logger:               logger.With("component", "cri-service"),
		imgClient:            imgClient,
	}
}

func (s *svc) EnsurePod(ctx context.Context, opts RunOptions) error {
	listPodsResp, err := s.ListPodSandbox(ctx, &runtimev1.ListPodSandboxRequest{
		Filter: &runtimev1.PodSandboxFilter{
			LabelSelector: map[string]string{
				LabelPodUID: opts.PodConfig.Metadata.Uid,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("list pod sandbox: %w", err)
	}

	if len(listPodsResp.Items) > 0 {
		return nil
	}

	runPodResp, err := s.RunPodSandbox(ctx, &runtimev1.RunPodSandboxRequest{
		Config: opts.PodConfig,
	})
	if err != nil {
		return fmt.Errorf("create pod: %w", err)
	}

	if _, err := s.EnsureImage(ctx, opts.ContainerConfig.Image.Image); err != nil {
		return fmt.Errorf("ensure image: %w", err)
	}

	opts.ContainerConfig.LogPath = fmt.Sprintf(
		"%s_%s",
		opts.PodConfig.Metadata.Namespace,
		opts.PodConfig.Metadata.Name,
	)

	opts.ContainerConfig.Metadata = &runtimev1.ContainerMetadata{
		Name: opts.PodConfig.Metadata.Name,
	}

	req := &runtimev1.CreateContainerRequest{
		PodSandboxId:  runPodResp.PodSandboxId,
		Config:        opts.ContainerConfig,
		SandboxConfig: opts.PodConfig,
	}

	s.logger.InfoContext(ctx,
		"no matching workload found, creating pod",
		"pod_uid", opts.PodConfig.Metadata.Uid,
		"pod_name", opts.PodConfig.Metadata.Name,
		"namespace", opts.PodConfig.Metadata.Namespace,
	)

	if _, err := s.RunContainer(ctx, req); err != nil {
		return fmt.Errorf("run container: %w", err)
	}

	return nil
}

func (s *svc) RunContainer(ctx context.Context, req *runtimev1.CreateContainerRequest) (string, error) {
	ctrResp, err := s.CreateContainer(ctx, req)
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}

	if _, err := s.StartContainer(ctx, &runtimev1.StartContainerRequest{
		ContainerId: ctrResp.ContainerId,
	}); err != nil {
		return "", fmt.Errorf("start container: %w", err)
	}

	return ctrResp.ContainerId, nil
}

// EnsureImage first calls ListImages then checks if the image is contained in the response.
// if this is not the case PullImage is being called.
func (s *svc) EnsureImage(ctx context.Context, imageURL string) (bool, error) {
	listResp, err := s.imgClient.ListImages(ctx, &runtimev1.ListImagesRequest{})
	if err != nil {
		return false, fmt.Errorf("list images: %w", err)
	}

	var img *runtimev1.Image
	for _, tmp := range listResp.Images {
		if slices.Contains(tmp.RepoTags, imageURL) {
			img = tmp
			break
		}
	}

	if img != nil {
		return false, nil
	}

	logger := s.logger.With("image", imageURL)
	logger.InfoContext(ctx, "pulling image")

	if _, err := s.imgClient.PullImage(ctx, &runtimev1.PullImageRequest{
		Image: &runtimev1.ImageSpec{
			Image: imageURL,
		},
	}); err != nil {
		return false, fmt.Errorf("pull image: %w", err)
	}

	return true, nil
}

// TODO: DeletePodAndContainers function
//       -> stop all containers
//       -> remove all containers
//       -> stop pod
//       -> remove pod
