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

	"github.com/google/uuid"
	workloadv1alpha1 "github.com/spacechunks/platform/api/platformd/workload/v1alpha1"
)

type Server struct {
	workloadv1alpha1.UnimplementedWorkloadServiceServer
	svc       Service
	portAlloc *PortAllocator
	store     Store
}

func NewServer(svc Service, alloc *PortAllocator, store Store) *Server {
	return &Server{
		svc:       svc,
		portAlloc: alloc,
		store:     store,
	}
}

func (s *Server) RunWorkload(
	ctx context.Context,
	req *workloadv1alpha1.RunWorkloadRequest,
) (*workloadv1alpha1.RunWorkloadResponse, error) {
	port, err := s.portAlloc.Allocate()
	if err != nil {
		return nil, fmt.Errorf("allocate port: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("create uuid: %w", err)
	}

	w := Workload{
		ID:                   id.String(),
		Name:                 req.Name,
		Image:                req.Image,
		Namespace:            req.Namespace,
		Hostname:             req.Hostname,
		Labels:               req.Labels,
		NetworkNamespaceMode: req.NetworkNamespaceMode,
		Port:                 port,
	}

	s.store.SaveWorkload(w)

	if err := s.svc.RunWorkload(ctx, w); err != nil {
		return nil, fmt.Errorf("run workload: %w", err)
	}

	// FIXME(yannic): if we have more objects create codec package
	//                which contains conversion logic from domain
	//                to grpc object
	//
	return &workloadv1alpha1.RunWorkloadResponse{
		Workload: &workloadv1alpha1.Workload{
			Id:                   w.ID,
			Name:                 w.Name,
			Image:                w.Image,
			Namespace:            w.Namespace,
			Hostname:             w.Hostname,
			Labels:               w.Labels,
			NetworkNamespaceMode: w.NetworkNamespaceMode,
			Port:                 uint32(port),
		},
	}, nil
}

func (s *Server) WorkloadStatus(
	ctx context.Context,
	req *workloadv1alpha1.WorkloadStatusRequest,
) (*workloadv1alpha1.WorkloadStatusResponse, error) {
	if req.Id == "" {
		return nil, fmt.Errorf("workload id required")
	}

	w := s.store.GetWorkload(req.Id)
	if w == nil {
		return nil, fmt.Errorf("workload not found")
	}

	return &workloadv1alpha1.WorkloadStatusResponse{
		Status: &workloadv1alpha1.WorkloadStatus{
			State: workloadv1alpha1.WorkloadState_UNKNOWN,
			Port:  uint32(w.Port),
		},
	}, nil
}
