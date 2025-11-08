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

	"github.com/google/uuid"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/user"
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
		return nil, apierrs.ErrInvalidName
	}

	// we allow the description to be empty, because
	// some things like bedwars for example do not
	// need a description.

	c := Chunk{
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Tags:        req.GetTags(),
		Owner:       user.User{}, // TODO: get user id from provided api token
	}

	ret, err := s.service.CreateChunk(ctx, c)
	if err != nil {
		return nil, err
	}

	return &chunkv1alpha1.CreateChunkResponse{
		Chunk: ChunkToTransport(ret),
	}, nil
}

func (s *Server) GetChunk(
	ctx context.Context,
	req *chunkv1alpha1.GetChunkRequest,
) (*chunkv1alpha1.GetChunkResponse, error) {
	if _, err := uuid.Parse(req.GetId()); err != nil {
		return nil, apierrs.ErrInvalidChunkID
	}

	c, err := s.service.GetChunk(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	return &chunkv1alpha1.GetChunkResponse{
		Chunk: ChunkToTransport(c),
	}, nil
}

func (s *Server) UpdateChunk(
	ctx context.Context,
	req *chunkv1alpha1.UpdateChunkRequest,
) (*chunkv1alpha1.UpdateChunkResponse, error) {
	if _, err := uuid.Parse(req.GetId()); err != nil {
		return nil, apierrs.ErrInvalidChunkID
	}

	c := Chunk{
		ID:          req.GetId(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Tags:        req.GetTags(),
	}

	ret, err := s.service.UpdateChunk(ctx, c)
	if err != nil {
		return nil, err
	}

	return &chunkv1alpha1.UpdateChunkResponse{
		Chunk: ChunkToTransport(ret),
	}, nil
}

func (s *Server) ListChunks(
	ctx context.Context,
	_ *chunkv1alpha1.ListChunksRequest,
) (*chunkv1alpha1.ListChunksResponse, error) {
	ret, err := s.service.ListChunks(ctx)
	if err != nil {
		return nil, err
	}

	transport := make([]*chunkv1alpha1.Chunk, 0, len(ret))
	for _, c := range ret {
		transport = append(transport, ChunkToTransport(c))
	}

	return &chunkv1alpha1.ListChunksResponse{
		Chunks: transport,
	}, nil
}

func (s *Server) CreateFlavor(
	ctx context.Context,
	req *chunkv1alpha1.CreateFlavorRequest,
) (*chunkv1alpha1.CreateFlavorResponse, error) {
	if _, err := uuid.Parse(req.GetChunkId()); err != nil {
		return nil, apierrs.ErrInvalidChunkID
	}

	if req.GetName() == "" {
		return nil, apierrs.ErrInvalidName
	}

	domain := Flavor{
		Name: req.GetName(),
	}

	created, err := s.service.CreateFlavor(ctx, req.GetChunkId(), domain)
	if err != nil {
		return nil, err
	}

	return &chunkv1alpha1.CreateFlavorResponse{
		Flavor: FlavorToTransport(created),
	}, nil
}

func (s *Server) CreateFlavorVersion(
	ctx context.Context,
	req *chunkv1alpha1.CreateFlavorVersionRequest,
) (*chunkv1alpha1.CreateFlavorVersionResponse, error) {
	domain := FlavorVersionToDomain(req.GetVersion())

	version, diff, err := s.service.CreateFlavorVersion(ctx, req.GetFlavorId(), domain)
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

func (s *Server) BuildFlavorVersion(
	ctx context.Context,
	req *chunkv1alpha1.BuildFlavorVersionRequest,
) (*chunkv1alpha1.BuildFlavorVersionResponse, error) {
	return &chunkv1alpha1.BuildFlavorVersionResponse{}, s.service.BuildFlavorVersion(ctx, req.GetFlavorVersionId())
}

func (s *Server) GetUploadURL(
	ctx context.Context,
	req *chunkv1alpha1.GetUploadURLRequest,
) (*chunkv1alpha1.GetUploadURLResponse, error) {
	if _, err := uuid.Parse(req.GetFlavorVersionId()); err != nil {
		return nil, apierrs.ErrInvalidChunkID
	}

	if req.GetTarballHash() == "" {
		return nil, apierrs.ErrInvalidHash
	}

	url, err := s.service.GetUploadURL(ctx, req.GetFlavorVersionId(), req.GetTarballHash())
	if err != nil {
		return nil, fmt.Errorf("upload url: %w", err)
	}

	return &chunkv1alpha1.GetUploadURLResponse{
		Url: url,
	}, nil
}

func (s *Server) GetSupportedMinecraftVersions(
	ctx context.Context,
	_ *chunkv1alpha1.GetSupportedMinecraftVersionsRequest,
) (*chunkv1alpha1.GetSupportedMinecraftVersionsResponse, error) {
	versions, err := s.service.GetSupportedMinecraftVersions(ctx)
	if err != nil {
		return nil, err
	}
	return &chunkv1alpha1.GetSupportedMinecraftVersionsResponse{
		Versions: versions,
	}, nil
}
