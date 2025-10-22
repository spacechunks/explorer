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

package checkpoint

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/uuid"
	checkpointv1alpha1 "github.com/spacechunks/explorer/api/platformd/checkpoint/v1alpha1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	checkpointv1alpha1.UnimplementedCheckpointServiceServer
	service Service
}

func NewServer(service Service) *Server {
	return &Server{
		service: service,
	}
}

func (s *Server) CreateCheckpoint(
	ctx context.Context,
	req *checkpointv1alpha1.CreateCheckpointRequest,
) (*checkpointv1alpha1.CreateCheckpointResponse, error) {
	ref, err := name.ParseReference(req.BaseImageUrl)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid image url: %v", err)
	}

	id, err := s.service.CreateCheckpoint(context.Background(), ref)
	if err != nil {
		return nil, err
	}

	return &checkpointv1alpha1.CreateCheckpointResponse{
		CheckpointId: id,
	}, nil
}

func (s *Server) CheckpointStatus(
	ctx context.Context,
	req *checkpointv1alpha1.CheckpointStatusRequest,
) (*checkpointv1alpha1.CheckpointStatusResponse, error) {
	if _, err := uuid.Parse(req.CheckpointId); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid checkpoint id: %v", err)
	}

	if s := s.service.CheckpointStatus(req.CheckpointId); s != nil && s.CheckpointStatus != nil {
		return &checkpointv1alpha1.CheckpointStatusResponse{
			Status: &checkpointv1alpha1.CheckpointStatus{
				State: checkpointv1alpha1.CheckpointState(
					checkpointv1alpha1.CheckpointState_value[string(s.CheckpointStatus.State)],
				),
				Message: s.CheckpointStatus.Message,
			},
		}, nil
	}

	return nil, status.Error(codes.NotFound, "status for checkpoint is not available anymore")
}
