package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	xds2 "github.com/spacechunks/explorer/platformd/proxy/xds"
)

type Service interface {
	CreateListeners(ctx context.Context, workloadID string, addr netip.Addr) error
	ApplyGlobalResources(ctx context.Context) error
	DeleteListeners(ctx context.Context, workloadID string) error
}

type proxyService struct {
	logger      *slog.Logger
	cfg         Config
	resourceMap xds2.Map
}

func NewService(logger *slog.Logger, cfg Config, resourceMap xds2.Map) Service {
	return &proxyService{
		logger:      logger,
		cfg:         cfg,
		resourceMap: resourceMap,
	}
}

// ApplyGlobalResources configures a resources group with globally
// used resources like:
//   - original destination cluster where all traffic originating
//     from the container destined to the outside world will be
//     routed to.
//   - dns cluster where dns traffic from all workloads will be
//     routed to.
func (s *proxyService) ApplyGlobalResources(ctx context.Context) error {
	rg := xds2.ResourceGroup{
		Clusters: []*clusterv3.Cluster{
			DNSClusterResource(),
			OriginalDstClusterResource(),
		},
	}
	if _, err := s.resourceMap.Put(ctx, "global", rg); err != nil {
		return fmt.Errorf("apply envoy config: %w", err)
	}
	return nil
}

// CreateListeners creates HTTP, TCP as well as UDP(DNS) and TCP(DNS) listeners for the provided
// workload. this will fail if the workload does not exist.
func (s *proxyService) CreateListeners(ctx context.Context, workloadID string, addr netip.Addr) error {
	wrg, err := WorkloadResources(
		workloadID,
		netip.AddrPortFrom(addr, HTTPPort),
		netip.AddrPortFrom(addr, TCPPort),
		OriginalDstClusterName,
	)
	if err != nil {
		return fmt.Errorf("create workload resources: %w", err)
	}

	drg, err := DNSListenerResourceGroup(DNSClusterName, netip.AddrPortFrom(addr, DNSPort), s.cfg.DNSUpstream)
	if err != nil {
		return fmt.Errorf("create dns resources: %w", err)
	}

	merged := xds2.ResourceGroup{}
	merged.Listeners = append(wrg.Listeners, drg.Listeners...)
	merged.Clusters = append(wrg.Clusters, drg.Clusters...)
	merged.CLAS = append(wrg.CLAS, drg.CLAS...)

	s.logger.InfoContext(ctx, "applying workload resources", "workload_id", workloadID)

	if _, err := s.resourceMap.Put(ctx, workloadID, merged); err != nil {
		return fmt.Errorf("apply envoy config: %w", err)
	}

	return nil
}

func (s *proxyService) DeleteListeners(ctx context.Context, workloadID string) error {
	s.logger.InfoContext(ctx, "deleting listeners", "workload_id", workloadID)
	if _, err := s.resourceMap.Del(ctx, workloadID); err != nil {
		return fmt.Errorf("delete workload resources: %w", err)
	}
	return nil
}
