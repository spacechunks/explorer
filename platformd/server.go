package platformd

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"

	"github.com/google/uuid"
	workloadv1alpha1 "github.com/spacechunks/explorer/api/platformd/workload/v1alpha1"
	"github.com/spacechunks/explorer/internal/datapath"
	proxy2 "github.com/spacechunks/explorer/platformd/proxy"
	xds2 "github.com/spacechunks/explorer/platformd/proxy/xds"
	workload2 "github.com/spacechunks/explorer/platformd/workload"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/hashicorp/go-multierror"
	proxyv1alpha1 "github.com/spacechunks/explorer/api/platformd/proxy/v1alpha1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type Server struct {
	logger *slog.Logger
}

func NewServer(logger *slog.Logger) *Server {
	return &Server{
		logger: logger,
	}
}

func (s *Server) Run(ctx context.Context, cfg Config) error {
	criConn, err := grpc.NewClient(cfg.CRIListenSock, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
		xdsCfg = cache.NewSnapshotCache(true, cache.IDHash{}, nil)
		wlSvc  = workload2.NewService(
			s.logger,
			runtimev1.NewRuntimeServiceClient(criConn),
			runtimev1.NewImageServiceClient(criConn),
		)
		proxySvc = proxy2.NewService(
			s.logger,
			proxy2.Config{
				DNSUpstream: dnsUpstream,
			},
			xds2.NewMap(proxyNodeID, xdsCfg),
		)

		mgmtServer  = grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
		proxyServer = proxy2.NewServer(proxySvc)
		wlServer    = workload2.NewServer(
			wlSvc,
			workload2.NewPortAllocator(30000, 40000),
			workload2.NewStore(),
		)
	)

	proxyv1alpha1.RegisterProxyServiceServer(mgmtServer, proxyServer)
	workloadv1alpha1.RegisterWorkloadServiceServer(mgmtServer, wlServer)
	xds2.CreateAndRegisterServer(ctx, s.logger, mgmtServer, xdsCfg)

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
		return fmt.Errorf("attach dnat bpf: %w", err)
	}

	if err := bpf.AddSNATTarget(0, ip, uint8(iface.Index)); err != nil {
		return fmt.Errorf("add snat target: %w", err)
	}

	if err := bpf.AttachAndPinGetsockopt(cfg.GetsockoptCGroup); err != nil {
		return fmt.Errorf("attach getsockopt: %w", err)
	}

	if err := proxySvc.ApplyGlobalResources(ctx); err != nil {
		return fmt.Errorf("apply global resources: %w", err)
	}

	// before we start our grpc services make sure our system workloads are running
	if err := wlSvc.EnsureWorkload(ctx, workload2.Workload{
		ID:                   uuid.New().String(),
		Name:                 "envoy",
		Image:                cfg.EnvoyImage,
		Namespace:            "system",
		NetworkNamespaceMode: int32(runtimev1.NamespaceMode_NODE),
		Labels:               workload2.SystemWorkloadLabels("envoy"),
		Args:                 []string{"-c /etc/envoy/config.yaml", "-l debug"},
		Mounts: []workload2.Mount{
			{
				HostPath:      "/etc/platformd/proxy.conf",
				ContainerPath: "/etc/envoy/config.yaml",
			},
		},
	}, workload2.SystemWorkloadLabels("envoy")); err != nil {
		return fmt.Errorf("ensure envoy: %w", err)
	}

	if err := wlSvc.EnsureWorkload(ctx, workload2.Workload{
		ID:                   uuid.New().String(),
		Name:                 "coredns",
		Image:                "docker.io/coredns/coredns",
		Namespace:            "system",
		NetworkNamespaceMode: int32(runtimev1.NamespaceMode_NODE),
		Labels:               workload2.SystemWorkloadLabels("coredns"),
		Args:                 []string{"-conf", "/etc/coredns/Corefile"},
		Mounts: []workload2.Mount{
			{
				HostPath:      "/etc/platformd/dns.conf",
				ContainerPath: "/etc/coredns/Corefile",
			},
		},
	}, workload2.SystemWorkloadLabels("coredns")); err != nil {
		return fmt.Errorf("ensure coredns: %w", err)
	}

	unixSock, err := net.Listen("unix", cfg.ManagementServerListenSock)
	if err != nil {
		return fmt.Errorf("failed to listen on unix socket: %v", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	var g multierror.Group
	g.Go(func() error {
		if err := mgmtServer.Serve(unixSock); err != nil {
			cancel()
			return fmt.Errorf("failed to serve mgmt server: %w", err)
		}
		return nil
	})

	<-ctx.Done()

	// add stop related code below

	mgmtServer.GracefulStop()
	g.Go(func() error {
		if err := criConn.Close(); err != nil {
			return fmt.Errorf("cri conn close: %w", err)
		}
		return nil
	})

	return g.Wait().ErrorOrNil()
}
