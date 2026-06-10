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
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/pagination"
	"github.com/spacechunks/explorer/internal/resource"
	"github.com/spacechunks/explorer/internal/resource/codec"
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
	// we allow the description to be empty, because
	// some things like bedwars for example do not
	// need a description.
	c := resource.Chunk{
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Tags:        req.GetTags(),
		Flavors:     make([]resource.Flavor, 0),
	}

	ret, err := s.service.CreateChunk(ctx, c)
	if err != nil {
		return nil, err
	}

	return &chunkv1alpha1.CreateChunkResponse{
		Chunk: codec.ChunkToTransport(ret),
	}, nil
}

func (s *Server) GetChunk(
	ctx context.Context,
	req *chunkv1alpha1.GetChunkRequest,
) (*chunkv1alpha1.GetChunkResponse, error) {
	c, err := s.service.GetChunk(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	return &chunkv1alpha1.GetChunkResponse{
		Chunk: codec.ChunkToTransport(c),
	}, nil
}

func (s *Server) UpdateChunk(
	ctx context.Context,
	req *chunkv1alpha1.UpdateChunkRequest,
) (*chunkv1alpha1.UpdateChunkResponse, error) {
	c := resource.Chunk{
		ID:          req.GetId(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Tags:        req.GetTags(),
	}

	//userID, err := userIDFromToken(ctx)
	//if err != nil {
	//	return nil, fmt.Errorf("user id from token: %w", err)
	//}

	ret, err := s.service.UpdateChunk(ctx, c)
	if err != nil {
		return nil, err
	}

	return &chunkv1alpha1.UpdateChunkResponse{
		Chunk: codec.ChunkToTransport(ret),
	}, nil
}

func (s *Server) ListChunks(
	ctx context.Context,
	req *chunkv1alpha1.ListChunksRequest,
) (*chunkv1alpha1.ListChunksResponse, error) {
	if req.GetPageSize() > pagination.MaxPageSize {
		return nil, apierrs.ErrInvalidPageSize
	}

	pageSize := pagination.ResolvePageSize(req.GetPageSize())
	afterID, err := pagination.DecodePageToken(req.GetPageToken())
	if err != nil {
		return nil, apierrs.ErrInvalidPageToken
	}

	ret, err := s.service.ListChunks(ctx, pageSize+1, afterID)
	if err != nil {
		return nil, err
	}

	nextPageToken := ""
	if len(ret) > pageSize {
		ret = ret[:pageSize]
		nextPageToken = pagination.EncodePageToken(ret[len(ret)-1].ID)
	}

	transport := make([]*chunkv1alpha1.Chunk, 0, len(ret))
	for _, c := range ret {
		transport = append(transport, codec.ChunkToTransport(c))
	}

	return &chunkv1alpha1.ListChunksResponse{
		Chunks:        transport,
		NextPageToken: nextPageToken,
	}, nil
}

func (s *Server) CreateFlavor(
	ctx context.Context,
	req *chunkv1alpha1.CreateFlavorRequest,
) (*chunkv1alpha1.CreateFlavorResponse, error) {
	domain := resource.Flavor{
		Name: req.GetName(),
	}

	created, err := s.service.CreateFlavor(ctx, req.GetChunkId(), domain)
	if err != nil {
		return nil, err
	}

	return &chunkv1alpha1.CreateFlavorResponse{
		Flavor: codec.FlavorToTransport(created),
	}, nil
}

func (s *Server) CreateFlavorVersion(
	ctx context.Context,
	req *chunkv1alpha1.CreateFlavorVersionRequest,
) (*chunkv1alpha1.CreateFlavorVersionResponse, error) {
	domain := resource.FlavorVersion{
		Version:          req.GetVersion(),
		MinecraftVersion: req.GetMinecraftVersion(),
		Hash:             req.GetHash(),
		FileHashes:       codec.FileHashSliceToDomain(req.GetFileHashes()),
		MinPlayers:       req.MinPlayers,
		MaxPlayers:       req.MaxPlayers,
	}

	version, diff, err := s.service.CreateFlavorVersion(ctx, req.GetFlavorId(), domain)
	if err != nil {
		return nil, err
	}

	return &chunkv1alpha1.CreateFlavorVersionResponse{
		Version:      codec.FlavorVersionToTransport(version),
		ChangedFiles: codec.FileHashSliceToTransport(diff.Changed),
		RemovedFiles: codec.FileHashSliceToTransport(diff.Removed),
		AddedFiles:   codec.FileHashSliceToTransport(diff.Added),
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
	url, err := s.service.GetUploadURL(ctx, req.GetFlavorVersionId(), req.GetTarballHash(), req.GetTarballSizeBytes())
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

func (s *Server) UploadThumbnail(
	ctx context.Context,
	req *chunkv1alpha1.UploadThumbnailRequest,
) (*chunkv1alpha1.UploadThumbnailResponse, error) {
	if err := s.service.UpdateThumbnail(ctx, req.ChunkId, req.Image); err != nil {
		return nil, err
	}
	return &chunkv1alpha1.UploadThumbnailResponse{}, nil
}

func (s *Server) DeleteFlavor(
	ctx context.Context,
	req *chunkv1alpha1.DeleteFlavorRequest,
) (*chunkv1alpha1.DeleteFlavorResponse, error) {
	if err := s.service.DeleteFlavor(ctx, req.Id); err != nil {
		return nil, fmt.Errorf("delete flavor: %w", err)
	}

	return &chunkv1alpha1.DeleteFlavorResponse{}, nil
}

func (s *Server) DeleteChunk(
	ctx context.Context,
	req *chunkv1alpha1.DeleteChunkRequest,
) (*chunkv1alpha1.DeleteChunkResponse, error) {
	if err := s.service.DeleteChunk(ctx, req.Id); err != nil {
		return nil, fmt.Errorf("delete chunk: %w", err)
	}

	return &chunkv1alpha1.DeleteChunkResponse{}, nil
}

func (s *Server) GetFlavor(
	ctx context.Context,
	req *chunkv1alpha1.GetFlavorRequest,
) (*chunkv1alpha1.GetFlavorResponse, error) {
	f, err := s.service.GetFlavor(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("get flavor: %w", err)
	}

	return &chunkv1alpha1.GetFlavorResponse{
		Flavor: codec.FlavorToTransport(f),
	}, nil
}
