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
	"fmt"

	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrInvalidChunkID = status.Errorf(codes.InvalidArgument, "chunk id is invalid")
	ErrInvalidName    = status.Errorf(codes.InvalidArgument, "name is invalid")
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

func (s *Server) CreateChunk(
	ctx context.Context,
	req *chunkv1alpha1.CreateChunkRequest,
) (*chunkv1alpha1.CreateChunkResponse, error) {
	if req.GetName() == "" {
		return nil, ErrInvalidName
	}

	// we allow the description to be empty, because
	// some things like bedwars for example do not
	// need a description.

	c := Chunk{
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Tags:        req.GetTags(),
	}

	ret, err := s.service.CreateChunk(ctx, c)
	if err != nil {
		return nil, err
	}

	return &chunkv1alpha1.CreateChunkResponse{
		Chunk: ChunkToTransport(ret),
	}, nil
}

func (s *Server) CreateFlavor(
	ctx context.Context,
	req *chunkv1alpha1.CreateFlavorRequest,
) (*chunkv1alpha1.CreateFlavorResponse, error) {
	if req.GetChunkId() == "" {
		return nil, ErrInvalidChunkID
	}

	if req.GetName() == "" {
		return nil, ErrInvalidName
	}

	domain := Flavor{
		Name: req.GetName(),
	}

	created, err := s.service.CreateFlavor(ctx, req.GetChunkId(), domain)
	if err != nil {
		return nil, fmt.Errorf("create flavor: %w", err)
	}

	return &chunkv1alpha1.CreateFlavorResponse{
		Flavor: FlavorToTransport(created),
	}, nil
}

func (s *Server) ListFlavors(
	ctx context.Context,
	req *chunkv1alpha1.ListFlavorsRequest,
) (*chunkv1alpha1.ListFlavorsResponse, error) {
	if req.GetChunkId() == "" {
		return nil, ErrInvalidChunkID
	}

	flavors, err := s.service.ListFlavors(ctx, req.GetChunkId())
	if err != nil {
		return nil, fmt.Errorf("create flavor: %w", err)
	}

	sl := make([]*chunkv1alpha1.Flavor, 0, len(flavors))
	for _, flavor := range flavors {
		sl = append(sl, FlavorToTransport(flavor))
	}

	return &chunkv1alpha1.ListFlavorsResponse{
		Flavors: sl,
	}, nil
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

func (s *Server) SaveFlavorFiles(
	ctx context.Context,
	req *chunkv1alpha1.SaveFlavorFilesRequest,
) (*chunkv1alpha1.SaveFlavorFilesResponse, error) {
	files := make([]File, 0, len(req.Files))
	for _, f := range req.Files {
		files = append(files, File{
			Path: f.GetPath(),
			Data: f.GetData(),
		})
	}

	if err := s.service.SaveFlavorFiles(ctx, req.GetFlavorVersionId(), files); err != nil {
		return &chunkv1alpha1.SaveFlavorFilesResponse{}, err
	}

	return &chunkv1alpha1.SaveFlavorFilesResponse{}, nil
}
