package cri

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

const PodLogDir = "/var/log/platformd/pods"

type RunOptions struct {
	PodConfig       *runtimev1.PodSandboxConfig
	ContainerConfig *runtimev1.ContainerConfig
}

type Service interface {
	GetRuntimeClient() runtimev1.RuntimeServiceClient
	EnsurePod(ctx context.Context, opts RunOptions, imageURL string) error
	RunContainer(ctx context.Context, req *runtimev1.CreateContainerRequest) (string, error)
	EnsureImage(ctx context.Context, imageURL string) error
}

type svc struct {
	logger    *slog.Logger
	rtClient  runtimev1.RuntimeServiceClient
	imgClient runtimev1.ImageServiceClient
}

func NewService(
	logger *slog.Logger,
	rtClient runtimev1.RuntimeServiceClient,
	imgClient runtimev1.ImageServiceClient,
) Service {
	return &svc{
		logger:    logger.With("component", "cri-service"),
		rtClient:  rtClient,
		imgClient: imgClient,
	}
}

func (s *svc) GetRuntimeClient() runtimev1.RuntimeServiceClient {
	return s.rtClient
}

func (s *svc) EnsurePod(ctx context.Context, opts RunOptions, imageURL string) error {
	resp, err := s.rtClient.ListPodSandbox(ctx, &runtimev1.ListPodSandboxRequest{
		Filter: &runtimev1.PodSandboxFilter{
			Id: opts.PodConfig.Metadata.Uid,
		},
	})
	if err != nil {
		return fmt.Errorf("list pod sandbox: %w", err)
	}

	// TODO: what do we do if the pod found is in NOT_READY state

	if len(resp.Items) > 0 {
		return nil
	}

	if _, err := s.rtClient.RunPodSandbox(ctx, &runtimev1.RunPodSandboxRequest{
		Config: opts.PodConfig,
	}); err != nil {
		return fmt.Errorf("create pod: %w", err)
	}

	opts.ContainerConfig.Image = &runtimev1.ImageSpec{
		UserSpecifiedImage: imageURL,
		Image:              imageURL,
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
		PodSandboxId:  opts.PodConfig.Metadata.Uid,
		Config:        opts.ContainerConfig,
		SandboxConfig: opts.PodConfig,
	}

	s.logger.InfoContext(ctx,
		"no matching workload found, creating pod",
		"pod_id", opts.PodConfig.Metadata.Uid,
		"pod_name", opts.PodConfig.Metadata.Name,
		"namespace", opts.PodConfig.Metadata.Namespace,
	)

	ctrResp, err := s.rtClient.CreateContainer(ctx, req)
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}

	if _, err := s.rtClient.StartContainer(ctx, &runtimev1.StartContainerRequest{
		ContainerId: ctrResp.ContainerId,
	}); err != nil {
		return fmt.Errorf("start container: %w", err)
	}

	return nil
}

func (s *svc) RunContainer(ctx context.Context, req *runtimev1.CreateContainerRequest) (string, error) {
	ctrResp, err := s.rtClient.CreateContainer(ctx, req)
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}

	if _, err := s.rtClient.StartContainer(ctx, &runtimev1.StartContainerRequest{
		ContainerId: ctrResp.ContainerId,
	}); err != nil {
		return "", fmt.Errorf("start container: %w", err)
	}

	return ctrResp.ContainerId, nil
}

// PullImageIfNotPresent first calls ListImages then checks if the image is contained in the response.
// if this is not the case PullImage is being called. this function does not access the services logger,
// and instead uses a passed one, to preserve arguments which provide additional context to the image pull.
func (s *svc) EnsureImage(ctx context.Context, imageURL string) error {
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

	logger := s.logger.With("image", imageURL)
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
