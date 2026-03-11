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

package workload

import (
	"context"
	"fmt"

	workloadv1alpha2 "github.com/spacechunks/explorer/api/platformd/workload/v1alpha2"
	"github.com/spacechunks/explorer/platformd/status"
)

type Server struct {
	workloadv1alpha2.UnimplementedWorkloadServiceServer
	store   status.Store
	service Service
}

func NewServer(store status.Store, service Service) *Server {
	return &Server{
		store:   store,
		service: service,
	}
}

func (s *Server) WorkloadStatus(
	_ context.Context,
	req *workloadv1alpha2.WorkloadStatusRequest,
) (*workloadv1alpha2.WorkloadStatusResponse, error) {
	if req.GetId() == "" {
		return nil, fmt.Errorf("workload id required")
	}

	domain := s.store.Get(req.GetId())
	if domain == nil {
		return nil, fmt.Errorf("workload not found")
	}

	transport := StatusToTransport(*domain)

	return &workloadv1alpha2.WorkloadStatusResponse{
		Status: transport,
	}, nil
}

// TODO: tests

func (s *Server) StopWorkload(
	ctx context.Context,
	req *workloadv1alpha2.WorkloadStopRequest,
) (*workloadv1alpha2.WorkloadStopResponse, error) {
	id := req.GetId()

	if id == "" {
		return nil, fmt.Errorf("workload id required")
	}

	if err := s.service.RemoveWorkload(ctx, id); err != nil {
		return nil, fmt.Errorf("remove workload: %w", err)
	}

	s.store.Update(id, status.Status{
		WorkloadStatus: &status.WorkloadStatus{
			State: status.WorkloadStateDeleted,
		},
	})

	return &workloadv1alpha2.WorkloadStopResponse{}, nil
}
