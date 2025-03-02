/*
 Explorer Platform, a platform for hosting and discovering Minecraft servers.
 Copyright (C) 2024 Yannic Rieger <oss@76k.io>

 This program is free software: you can redistribute it and/or modify
 it under the terms of the GNU Affero General Public License as published by
 the Free Software Foundation, either version 3 of the License, or
 (at your option) any later version.

 This program is distributed in the hope that it will be useful,
 but WITHOUT ANY WARRANTY; without even the implied warranty of
 MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 GNU Affero General Public License for more details.

 You should have received a copy of the GNU Affero General Public License
 along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package discovery

import (
	"context"
	"fmt"

	discoveryv1alpha1 "github.com/spacechunks/explorer/api/discovery/v1alpha1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	discoveryv1alpha1.UnimplementedDiscoveryServiceServer
	svc Service
}

func NewServer(service Service) *Server {
	return &Server{
		svc: service,
	}
}

func (s *Server) DiscoverWorkloads(
	ctx context.Context,
	req *discoveryv1alpha1.DiscoverWorkloadRequest,
) (*discoveryv1alpha1.DiscoverWorkloadResponse, error) {
	if req.GetNodeKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "node key is required")
	}

	wls, err := s.svc.DiscoverWorkloads(ctx, req.GetNodeKey())
	if err != nil {
		return nil, fmt.Errorf("discovering workloads: %w", err)
	}

	return &discoveryv1alpha1.DiscoverWorkloadResponse{
		Workloads: wls,
	}, nil
}

func (s *Server) ReceiveWorkloadStatusReports(
	context.Context,
	*discoveryv1alpha1.ReceiveWorkloadStatusReportsRequest,
) (*discoveryv1alpha1.ReceiveWorkloadStatusReportResponse, error) {
	return &discoveryv1alpha1.ReceiveWorkloadStatusReportResponse{}, nil
}
