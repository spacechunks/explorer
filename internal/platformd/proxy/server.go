package proxy

import (
	"context"
	"fmt"
	"net/netip"

	proxyv1alpha1 "github.com/spacechunks/platform/api/platformd/proxy/v1alpha1"
)

type Server struct {
	proxyv1alpha1.UnimplementedProxyServiceServer
	svc Service
}

func NewServer(svc Service) *Server {
	return &Server{
		svc: svc,
	}
}

func (s *Server) CreateListeners(
	ctx context.Context,
	req *proxyv1alpha1.CreateListenersRequest,
) (*proxyv1alpha1.CreateListenersResponse, error) {
	addr, err := netip.ParseAddr(req.Ip)
	if err != nil {
		return nil, fmt.Errorf("parse addr: %w", err)
	}

	// TODO: if workload does not exist return err

	if err := s.svc.CreateListeners(ctx, req.WorkloadID, addr); err != nil {
		return nil, fmt.Errorf("create listener: %w", err)
	}

	return &proxyv1alpha1.CreateListenersResponse{}, nil
}
