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
)

type Service interface {
	RunChunk(ctx context.Context, id string) (Instance, error)
	CreateChunk(ctx context.Context, chunk Chunk) (Chunk, error)
}

type svc struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &svc{
		repo: repo,
	}
}

func (s *svc) RunChunk(ctx context.Context, id string) (Instance, error) {
	_, err := s.repo.GetChunkByID(ctx, id)
	if err != nil {
		return Instance{}, fmt.Errorf("chunk by id: %w", err)
	}
	// TODO: create workload
	// TODO: return instance
	return Instance{}, nil
}

func (s *svc) CreateChunk(ctx context.Context, chunk Chunk) (Chunk, error) {
	ret, err := s.repo.CreateChunk(ctx, chunk)
	if err != nil {
		return Chunk{}, fmt.Errorf("create chunk: %w", err)
	}
	return ret, nil
}
