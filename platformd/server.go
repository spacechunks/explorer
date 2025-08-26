package platformd

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/hashicorp/go-multierror"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	"github.com/spacechunks/explorer/internal/image"
	"github.com/spacechunks/explorer/platformd/garbage"
	"google.golang.org/grpc/credentials"
	"k8s.io/client-go/tools/remotecommand"

	//instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	checkpointv1alpha1 "github.com/spacechunks/explorer/api/platformd/checkpoint/v1alpha1"
	proxyv1alpha1 "github.com/spacechunks/explorer/api/platformd/proxy/v1alpha1"
	workloadv1alpha2 "github.com/spacechunks/explorer/api/platformd/workload/v1alpha2"
	"github.com/spacechunks/explorer/internal/datapath"
	"github.com/spacechunks/explorer/platformd/checkpoint"
	"github.com/spacechunks/explorer/platformd/cri"
	"github.com/spacechunks/explorer/platformd/proxy"
	"github.com/spacechunks/explorer/platformd/proxy/xds"
	"github.com/spacechunks/explorer/platformd/workload"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type Server struct {
	logger *slog.Logger
	stopCh chan struct{}
}

func NewServer(logger *slog.Logger) *Server {
	return &Server{
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

func (s *Server) Run(ctx context.Context, cfg Config) error {
	s.logger.Info("started with config", "config", cfg)

	tlsCreds := credentials.NewTLS(&tls.Config{
		InsecureSkipVerify: true,
	})

	criConn, err := grpc.NewClient(cfg.CRIListenSock, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to create cri grpc client: %w", err)
	}

	cpConn, err := grpc.NewClient(cfg.ControlPlaneEndpoint, grpc.WithTransportCredentials(tlsCreds))
	if err != nil {
		return fmt.Errorf("failed to create cri grpc client: %w", err)
	}

	dnsUpstream, err := netip.ParseAddrPort(cfg.DNSServer)
	if err != nil {
		return fmt.Errorf("failed to parse dns server address: %w", err)
	}

	// hardcode envoy node id here, because implementing support
	// for multiple proxy nodes is currently way out of scope
	// and might not even be needed at all. being configurable
	// at the current time is also not needed, due to being a
	// wholly internal property that is not expected to be changed
	// by end users.
	//
	// note that if the node id in proxy.conf differs from this one,
	// configuring the proxy will fail and network connectivity will
	// be impaired for the workloads.
	const proxyNodeID = "proxy-0"

	var (
		xdsCfg    = cache.NewSnapshotCache(true, cache.IDHash{}, nil)
		insClient = instancev1alpha1.NewInstanceServiceClient(cpConn)
		criSvc    = cri.NewService(
			s.logger.With("component", "cri-service"),
			runtimev1.NewRuntimeServiceClient(criConn),
			runtimev1.NewImageServiceClient(criConn),
		)
		registryAuth = cri.RegistryAuth{
			Username: cfg.RegistryUser,
			Password: cfg.RegistryPass,
		}
		wlSvc = workload.NewService(
			s.logger,
			criSvc,
			registryAuth,
		)
		proxySvc = proxy.NewService(
			s.logger.With("component", "proxy-service"),
			proxy.Config{
				DNSUpstream: dnsUpstream,
			},
			xds.NewMap(proxyNodeID, xdsCfg),
		)

		checkSvcLogger = s.logger.With("component", "checkpoint-service")
		checkSvc       = checkpoint.NewService(
			checkSvcLogger,
			checkpoint.Config{
				CPUPeriod:                cfg.CheckpointConfig.CPUPeriod,
				CPUQuota:                 cfg.CheckpointConfig.CPUQuota,
				MemoryLimitBytes:         cfg.CheckpointConfig.MemoryLimitBytes,
				CheckpointFileDir:        cfg.CheckpointConfig.CheckpointFileDir,
				CheckpointTimeoutSeconds: cfg.CheckpointConfig.CheckpointTimeoutSeconds,
				RegistryUser:             cfg.CheckpointConfig.RegistryUser,
				RegistryPass:             cfg.CheckpointConfig.RegistryPass,
				ListenAddr:               cfg.CheckpointConfig.ListenAddr,
				StatusRetentionPeriod:    cfg.CheckpointConfig.StatusRetentionPeriod,
				ContainerReadyTimeout:    cfg.CheckpointConfig.ContainerReadyTimeout,
			},
			criSvc,
			image.NewService(checkSvcLogger, cfg.RegistryUser, cfg.RegistryPass, "/tmp"),
			checkpoint.NewStore(),
			func(url string) (remotecommand.Executor, error) {
				return checkpoint.NewSPDYExecutor(url)
			},
		)

		proxyServer = proxy.NewServer(proxySvc)
		wlStore     = workload.NewStore()
		wlServer    = workload.NewServer(wlStore)
		checkServer = checkpoint.NewServer(checkSvc)
		reconciler  = newReconciler(s.logger, reconcilerConfig{
			MaxAttempts:       cfg.MaxAttempts,
			SyncInterval:      cfg.SyncInterval,
			NodeID:            cfg.NodeID,
			MinPort:           cfg.MinPort,
			MaxPort:           cfg.MaxPort,
			WorkloadNamespace: cfg.WorkloadNamespace,
			RegistryEndpoint:  cfg.RegistryEndpoint,
		}, insClient, wlSvc, wlStore)
		gc = garbage.NewExecutor(s.logger, 1*time.Second, checkSvc)
	)

	mgmtServer := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	proxyv1alpha1.RegisterProxyServiceServer(mgmtServer, proxyServer)
	workloadv1alpha2.RegisterWorkloadServiceServer(mgmtServer, wlServer)
	checkpointv1alpha1.RegisterCheckpointServiceServer(mgmtServer, checkServer)
	xds.CreateAndRegisterServer(ctx, s.logger.With("component", "xds"), mgmtServer, xdsCfg)

	checkGRPCServer := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	checkpointv1alpha1.RegisterCheckpointServiceServer(checkGRPCServer, checkServer)

	bpf, err := datapath.LoadBPF()
	if err != nil {
		return fmt.Errorf("failed to load bpf: %w", err)
	}

	iface, err := net.InterfaceByName(cfg.HostIface)
	if err != nil {
		return fmt.Errorf("failed to get host interface: %w", err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return fmt.Errorf("failed to get addresses: %w", err)
	}

	ip, _, err := net.ParseCIDR(addrs[0].String())
	if err != nil {
		return fmt.Errorf("failed to parse ip: %w", err)
	}

	if err := bpf.AttachAndPinDNAT(iface); err != nil {
		return fmt.Errorf("attach dnat bpf: %w", err) // TODO: ignore exists, FIXME: update if exists
	}

	if err := bpf.AddSNATTarget(0, ip, uint8(iface.Index)); err != nil {
		return fmt.Errorf("add snat target: %w", err)
	}

	if err := bpf.AttachAndPinGetsockopt(cfg.GetsockoptCGroup); err != nil {
		return fmt.Errorf("attach getsockopt: %w", err) // TODO: ignore exists, FIXME: update if exists
	}

	if err := proxySvc.ApplyGlobalResources(ctx); err != nil {
		return fmt.Errorf("apply global resources: %w", err)
	}

	if err := os.MkdirAll(cfg.CheckpointConfig.CheckpointFileDir, 0777); err != nil {
		return fmt.Errorf("create checkpoint location dir: %w", err)
	}

	// before we start our grpc services make sure our system workloads are running

	if err := criSvc.EnsurePod(ctx, cri.RunOptions{
		PodConfig: &runtimev1.PodSandboxConfig{
			Metadata: &runtimev1.PodSandboxMetadata{
				Uid:       "envoy",
				Name:      "envoy",
				Namespace: "explorer-system",
			},
			Hostname:     "envoy",
			LogDirectory: cri.PodLogDir,
			Linux: &runtimev1.LinuxPodSandboxConfig{
				SecurityContext: &runtimev1.LinuxSandboxSecurityContext{
					NamespaceOptions: &runtimev1.NamespaceOption{
						Network: runtimev1.NamespaceMode_NODE,
					},
				},
				// TODO: resources
			},
		},
		ContainerConfig: &runtimev1.ContainerConfig{
			Args: []string{"-c /etc/envoy/config.yaml", "-l debug"},
			Image: &runtimev1.ImageSpec{
				Image:              cfg.EnvoyImage,
				UserSpecifiedImage: cfg.EnvoyImage,
			},
			Mounts: []*runtimev1.Mount{
				{
					HostPath:      "/etc/platformd/proxy.conf",
					ContainerPath: "/etc/envoy/config.yaml",
				},
			},
		},
	}); err != nil {
		return fmt.Errorf("ensure envoy: %w", err)
	}

	if err := criSvc.EnsurePod(ctx, cri.RunOptions{
		PodConfig: &runtimev1.PodSandboxConfig{
			Metadata: &runtimev1.PodSandboxMetadata{
				Uid:       "coredns",
				Name:      "coredns",
				Namespace: "explorer-system",
			},
			Labels:       workload.SystemWorkloadLabels("coredns"),
			Hostname:     "coredns",
			LogDirectory: cri.PodLogDir,
			Linux: &runtimev1.LinuxPodSandboxConfig{
				SecurityContext: &runtimev1.LinuxSandboxSecurityContext{
					NamespaceOptions: &runtimev1.NamespaceOption{
						Network: runtimev1.NamespaceMode_NODE,
					},
				},
				// TODO: resources
			},
		},
		ContainerConfig: &runtimev1.ContainerConfig{
			Args: []string{"-conf", "/etc/coredns/Corefile"},
			Image: &runtimev1.ImageSpec{
				Image:              cfg.CoreDNSImage,
				UserSpecifiedImage: cfg.CoreDNSImage,
			},
			Mounts: []*runtimev1.Mount{
				{
					HostPath:      "/etc/platformd/dns.conf",
					ContainerPath: "/etc/coredns/Corefile",
				},
			},
		},
	}); err != nil {
		return fmt.Errorf("ensure coredns: %w", err)
	}

	var g multierror.Group

	g.Go(func() error {
		checkSock, err := net.Listen("tcp", cfg.CheckpointConfig.ListenAddr)
		if err != nil {
			s.stopCh <- struct{}{}
			return fmt.Errorf("failed to listen on unix socket: %v", err)
		}

		if err := checkGRPCServer.Serve(checkSock); err != nil {
			s.stopCh <- struct{}{}
			return fmt.Errorf("failed to serve mgmt server: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		unixSock, err := net.Listen("unix", cfg.ManagementServerListenSock)
		if err != nil {
			s.stopCh <- struct{}{}
			return fmt.Errorf("failed to listen on unix socket: %v", err)
		}

		if err := mgmtServer.Serve(unixSock); err != nil {
			s.stopCh <- struct{}{}
			return fmt.Errorf("failed to serve mgmt server: %w", err)
		}
		return nil
	})

	// start reconciler after mgmt server has been started,
	// because otherwise creating pending instances will
	// fail as netglue is not able to retrieve the allocated
	// host port.
	go gc.Run(ctx)
	go reconciler.Start(ctx)

	<-s.stopCh

	// add stop related code below

	mgmtServer.Stop()
	checkGRPCServer.Stop()

	gc.Stop()
	reconciler.Stop()

	g.Go(func() error {
		if err := criConn.Close(); err != nil {
			return fmt.Errorf("cri conn close: %w", err)
		}
		return nil
	})

	return g.Wait().ErrorOrNil()
}

func (s *Server) Stop() {
	s.stopCh <- struct{}{}
}
