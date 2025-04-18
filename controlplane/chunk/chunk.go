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
	"unicode/utf8"

	"github.com/google/uuid"
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

func (s *svc) CreateChunk(ctx context.Context, chunk Chunk) (Chunk, error) {
	// FIXME:
	//  - flavor limit
	//  - remove hardcoded limits for tags

	if len(chunk.Tags) > MaxChunkTags {
		return Chunk{}, ErrTooManyTags
	}

	if utf8.RuneCountInString(chunk.Name) > MaxChunkNameChars {
		return Chunk{}, ErrNameTooLong
	}

	if utf8.RuneCountInString(chunk.Description) > MaxChunkDescriptionChars {
		return Chunk{}, ErrDescriptionTooLong
	}

	// FIXME: move id generation to repo
	id, err := uuid.NewV7()
	if err != nil {
		return Chunk{}, fmt.Errorf("generate id: %w", err)
	}

	chunk.ID = id.String()

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
