package workload

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/google/uuid"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type Workload struct {
	ID                   string
	Name                 string
	Image                string
	Namespace            string
	Hostname             string
	Labels               map[string]string
	NetworkNamespaceMode int32
}

type Mount struct {
	ContainerPath string
	HostPath      string
}

type RunOptions struct {
	Name      string
	Image     string
	Namespace string
	Hostname  string
	Labels    map[string]string

	// NetworkNamespaceMode as per [runtimev1.NamespaceMode].
	// keeping this value an int32 is intentional, so the workload
	// api does not rely on runtime version specific value mapping,
	// which would be the case if we were defining enum values for each
	// [runtimev1.NamespaceMode] value.
	NetworkNamespaceMode int32

	Mounts    []Mount
	Args      []string
	DNSServer string
}

const podLogDir = "/var/log/platformd/pods"

type Service interface {
	RunWorkload(ctx context.Context, opts RunOptions) (Workload, error)
	EnsureWorkload(ctx context.Context, opts RunOptions, labelSelector map[string]string) error
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
		logger:    logger,
		rtClient:  rtClient,
		imgClient: imgClient,
	}
}

// EnsureWorkload ensures that a pod is created if not present.
// if ListPodSandbox returns 0 items, a pod with the passed configuration is created.
// Currently, this function is designed for a single item returned by the label selector.
// If multiple items are returned the first one will be picked.
func (s *criService) EnsureWorkload(ctx context.Context, opts RunOptions, labelSelector map[string]string) error {
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
		"pod_name", opts.Name,
		"namespace", opts.Namespace,
		"label_selector", labelSelector,
	)

	if _, err := s.RunWorkload(ctx, opts); err != nil {
		return fmt.Errorf("create pod: %w", err)
	}
	return nil
}

// RunWorkload calls the CRI to create a new pod defined by [RunOptions].
// returns the generated uuidv7 ID of the workload. this id is also used in the
// pods metadata uid field.
func (s *criService) RunWorkload(ctx context.Context, opts RunOptions) (Workload, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Workload{}, fmt.Errorf("new uuid: %w", err)
	}

	logger := s.logger.With("workload_id", id.String(), "pod_name", opts.Name, "namespace", opts.Namespace)

	if err := s.pullImageIfNotPresent(ctx, logger, opts.Image); err != nil {
		return Workload{}, fmt.Errorf("pull image if not present: %w", err)
	}

	sboxCfg := &runtimev1.PodSandboxConfig{
		Metadata: &runtimev1.PodSandboxMetadata{
			Name:      opts.Name,
			Uid:       id.String(),
			Namespace: opts.Namespace,
		},
		Hostname:     opts.Hostname, // TODO: explore if we can use the id as the hostname
		LogDirectory: podLogDir,
		Labels:       opts.Labels,
		DnsConfig: &runtimev1.DNSConfig{
			Servers:  []string{opts.DNSServer},
			Options:  []string{"edns0", "trust-ad"},
			Searches: []string{"."},
		},
		Linux: &runtimev1.LinuxPodSandboxConfig{
			SecurityContext: &runtimev1.LinuxSandboxSecurityContext{
				NamespaceOptions: &runtimev1.NamespaceOption{
					Network: runtimev1.NamespaceMode(opts.NetworkNamespaceMode),
				},
			},
		},
	}

	sboxResp, err := s.rtClient.RunPodSandbox(ctx, &runtimev1.RunPodSandboxRequest{
		Config: sboxCfg,
	})
	if err != nil {
		return Workload{}, fmt.Errorf("create pod: %w", err)
	}

	logger = logger.With("pod_id", sboxResp.PodSandboxId)
	logger.InfoContext(ctx, "started pod sandbox")

	req := &runtimev1.CreateContainerRequest{
		PodSandboxId: sboxResp.PodSandboxId,
		Config: &runtimev1.ContainerConfig{
			Metadata: &runtimev1.ContainerMetadata{
				Name:    opts.Name,
				Attempt: 0,
			},
			Image: &runtimev1.ImageSpec{
				UserSpecifiedImage: opts.Image,
				Image:              opts.Image,
			},
			Labels:  opts.Labels,
			LogPath: fmt.Sprintf("%s_%s", opts.Namespace, opts.Name),
			Args:    opts.Args,
		},
		SandboxConfig: sboxCfg,
	}

	mnts := make([]*runtimev1.Mount, 0, len(opts.Mounts))
	for _, m := range opts.Mounts {
		mnts = append(mnts, &runtimev1.Mount{
			ContainerPath: m.ContainerPath,
			HostPath:      m.HostPath,
		})
	}

	req.Config.Mounts = mnts

	ctrResp, err := s.rtClient.CreateContainer(ctx, req)
	if err != nil {
		return Workload{}, fmt.Errorf("create container: %w", err)
	}

	if _, err := s.rtClient.StartContainer(ctx, &runtimev1.StartContainerRequest{
		ContainerId: ctrResp.ContainerId,
	}); err != nil {
		return Workload{}, fmt.Errorf("start container: %w", err)
	}

	logger.InfoContext(ctx, "started container", "container_id", ctrResp.ContainerId)
	return Workload{
		ID:                   id.String(),
		Name:                 opts.Name,
		Image:                opts.Image,
		Namespace:            opts.Namespace,
		Hostname:             opts.Hostname,
		Labels:               opts.Labels,
		NetworkNamespaceMode: opts.NetworkNamespaceMode,
	}, nil
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
