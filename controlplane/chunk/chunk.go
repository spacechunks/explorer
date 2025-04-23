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
	"time"
	"unicode/utf8"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrChunkNotFound      = status.Error(codes.NotFound, "chunk does not exist")
	ErrTooManyTags        = status.Error(codes.InvalidArgument, "too many tags")
	ErrNameTooLong        = status.Error(codes.InvalidArgument, "name is too long")
	ErrDescriptionTooLong = status.Error(codes.InvalidArgument, "description is too long")
)

const (
	MaxChunkTags             = 4
	MaxChunkNameChars        = 50
	MaxChunkDescriptionChars = 100
)

type Chunk struct {
	ID          string
	Name        string
	Description string
	Tags        []string
	Flavors     []Flavor
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (s *svc) CreateChunk(ctx context.Context, chunk Chunk) (Chunk, error) {
	if err := validateChunkFields(chunk); err != nil {
		return Chunk{}, err
	}

	ret, err := s.repo.CreateChunk(ctx, chunk)
	if err != nil {
		return Chunk{}, err
	}
	return ret, nil
}

func (s *svc) GetChunk(ctx context.Context, id string) (Chunk, error) {
	c, err := s.repo.GetChunkByID(ctx, id)
	if err != nil {
		return Chunk{}, err
	}
	return c, nil
}

func (s *svc) UpdateChunk(ctx context.Context, new Chunk) (Chunk, error) {
	if err := validateChunkFields(new); err != nil {
		return Chunk{}, err
	}

	old, err := s.repo.GetChunkByID(ctx, new.ID)
	if err != nil {
		return Chunk{}, fmt.Errorf("get chunk: %w", err)
	}

	if new.Name != "" {
		old.Name = new.Name
	}

	if new.Description != "" {
		old.Description = new.Description
	}

	if new.Tags != nil {
		old.Tags = new.Tags
	}

	ret, err := s.repo.UpdateChunk(ctx, old)
	if err != nil {
		return Chunk{}, fmt.Errorf("update chunk: %w", err)
	}

	return ret, nil
}

func (s *svc) ListChunks(ctx context.Context) ([]Chunk, error) {
	ret, err := s.repo.ListChunks(ctx)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func validateChunkFields(chunk Chunk) error {
	// FIXME:
	//  - remove hardcoded limits for tags

	if len(chunk.Tags) > MaxChunkTags {
		return ErrTooManyTags
	}

	if utf8.RuneCountInString(chunk.Name) > MaxChunkNameChars {
		return ErrNameTooLong
	}

	if utf8.RuneCountInString(chunk.Description) > MaxChunkDescriptionChars {
		return ErrDescriptionTooLong
	}

	return nil
}
