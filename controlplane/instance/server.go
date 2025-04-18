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

package instance

import (
	"context"
	"fmt"

	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrInvalidInstanceID = status.Errorf(codes.InvalidArgument, "invalid instance id")

type Server struct {
	instancev1alpha1.UnimplementedInstanceServiceServer
	service Service
}

func NewServer(service Service) *Server {
	return &Server{
		service: service,
	}
}

func (s *Server) GetInstance(
	ctx context.Context,
	req *instancev1alpha1.GetInstanceRequest,
) (*instancev1alpha1.GetInstanceResponse, error) {
	if req.GetId() == "" {
		return nil, ErrInvalidInstanceID
	}

	ins, err := s.service.GetInstance(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &instancev1alpha1.GetInstanceResponse{
		Instance: ToTransport(ins),
	}, nil
}

func (s *Server) ListInstances(
	ctx context.Context,
	_ *instancev1alpha1.ListInstancesRequest,
) (*instancev1alpha1.ListInstancesResponse, error) {
	instances, err := s.service.ListInstances(ctx)
	if err != nil {
		return nil, err
	}

	transport := make([]*instancev1alpha1.Instance, 0, len(instances))
	for _, ins := range instances {
		transport = append(transport, ToTransport(ins))
	}

	return &instancev1alpha1.ListInstancesResponse{
		Instances: transport,
	}, nil
}

func (s *Server) RunChunk(
	ctx context.Context,
	req *instancev1alpha1.RunChunkRequest,
) (*instancev1alpha1.RunChunkResponse, error) {
	ins, err := s.service.RunChunk(ctx, req.GetChunkId(), req.GetFlavorId())
	if err != nil {
		return nil, fmt.Errorf("run chunk: %w", err)
	}

	return &instancev1alpha1.RunChunkResponse{
		Instance: ToTransport(ins),
	}, nil
}

func (s *Server) DiscoverInstances(
	ctx context.Context,
	req *instancev1alpha1.DiscoverInstanceRequest,
) (*instancev1alpha1.DiscoverInstanceResponse, error) {
	if req.GetNodeKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "node key is required")
	}

	instances, err := s.service.DiscoverInstances(ctx, req.GetNodeKey())
	if err != nil {
		return nil, fmt.Errorf("discovering instances: %w", err)
	}

	ret := make([]*instancev1alpha1.Instance, 0, len(instances))
	for _, ins := range instances {
		ret = append(ret, ToTransport(ins))
	}

	return &instancev1alpha1.DiscoverInstanceResponse{
		Instances: ret,
	}, nil
}

func (s *Server) ReceiveInstanceStatusReports(
	ctx context.Context,
	req *instancev1alpha1.ReceiveInstanceStatusReportsRequest,
) (*instancev1alpha1.ReceiveInstanceStatusReportsResponse, error) {
	reports := make([]StatusReport, 0, len(req.GetReports()))
	for _, r := range req.GetReports() {
		reports = append(reports, StatusReportToDomain(r))
	}

	if err := s.service.ReceiveInstanceStatusReports(ctx, reports); err != nil {
		return nil, fmt.Errorf("receive instance status reports: %w", err)
	}

	return &instancev1alpha1.ReceiveInstanceStatusReportsResponse{}, nil
}

// TODO: tests
