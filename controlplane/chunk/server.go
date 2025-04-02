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

package chunk

import (
	"context"

	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
)

type Server struct {
	chunkv1alpha1.UnimplementedChunkServiceServer
	service Service
}

func NewServer(service Service) *Server {
	return &Server{
		service: service,
	}
}

func (s *Server) CreateFlavorVersion(
	ctx context.Context,
	req *chunkv1alpha1.CreateFlavorVersionRequest,
) (*chunkv1alpha1.CreateFlavorVersionResponse, error) {
	domain := FlavorVersionToDomain(req.GetVersion())

	version, diff, err := s.service.CreateFlavorVersion(ctx, domain)
	if err != nil {
		return nil, err
	}

	return &chunkv1alpha1.CreateFlavorVersionResponse{
		Version:      FlavorVersionToTransport(version),
		ChangedFiles: FileHashSliceToTransport(diff.Changed),
		RemovedFiles: FileHashSliceToTransport(diff.Removed),
		AddedFiles:   FileHashSliceToTransport(diff.Added),
	}, nil
}
