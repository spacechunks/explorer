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
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/spacechunks/explorer/controlplane/authz"
	"github.com/spacechunks/explorer/controlplane/contextkeys"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/resource"
)

func (s *svc) CreateChunk(ctx context.Context, chunk resource.Chunk) (resource.Chunk, error) {
	if err := validateChunkFields(chunk); err != nil {
		return resource.Chunk{}, err
	}

	actorID, ok := ctx.Value(contextkeys.ActorID).(string)
	if !ok {
		return resource.Chunk{}, errors.New("actor_id not found in context")
	}

	if err := s.access.AccessAuthorized(
		ctx,
		authz.WithOwnershipRule(actorID, authz.ChunkResourceDef(chunk.ID)),
	); err != nil {
		return resource.Chunk{}, fmt.Errorf("access: %w", err)
	}

	chunk.Owner.ID = actorID

	ret, err := s.repo.CreateChunk(ctx, chunk)
	if err != nil {
		return resource.Chunk{}, err
	}
	return ret, nil
}

func (s *svc) GetChunk(ctx context.Context, id string) (resource.Chunk, error) {
	c, err := s.repo.GetChunkByID(ctx, id)
	if err != nil {
		return resource.Chunk{}, err
	}
	return c, nil
}

func (s *svc) UpdateChunk(ctx context.Context, new resource.Chunk) (resource.Chunk, error) {
	if err := validateChunkFields(new); err != nil {
		return resource.Chunk{}, err
	}

	old, err := s.repo.GetChunkByID(ctx, new.ID)
	if err != nil {
		return resource.Chunk{}, fmt.Errorf("get chunk: %w", err)
	}

	actorID, ok := ctx.Value(contextkeys.ActorID).(string)
	if !ok {
		return resource.Chunk{}, errors.New("actor_id not found in context")
	}

	if old.Owner.ID != actorID {
		return resource.Chunk{}, apierrs.ErrPermissionDenied
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
		return resource.Chunk{}, fmt.Errorf("update chunk: %w", err)
	}

	return ret, nil
}

func (s *svc) ListChunks(ctx context.Context) ([]resource.Chunk, error) {
	ret, err := s.repo.ListChunks(ctx)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (s *svc) GetSupportedMinecraftVersions(ctx context.Context) ([]string, error) {
	return s.repo.SupportedMinecraftVersions(ctx)
}

func validateChunkFields(chunk resource.Chunk) error {
	// FIXME:
	//  - remove hardcoded limits for tags

	if len(chunk.Tags) > resource.MaxChunkTags {
		return apierrs.ErrTooManyTags
	}

	if utf8.RuneCountInString(chunk.Name) > resource.MaxChunkNameChars {
		return apierrs.ErrNameTooLong
	}

	if utf8.RuneCountInString(chunk.Description) > resource.MaxChunkDescriptionChars {
		return apierrs.ErrDescriptionTooLong
	}

	return nil
}
