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
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"unicode/utf8"

	"github.com/spacechunks/explorer/controlplane/authz"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/contextkey"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/resource"

	_ "image/png"
)

func (s *svc) CreateChunk(ctx context.Context, chunk resource.Chunk) (resource.Chunk, error) {
	if err := validateChunkFields(chunk); err != nil {
		return resource.Chunk{}, err
	}

	actorID, ok := ctx.Value(contextkey.ActorID).(string)
	if !ok {
		return resource.Chunk{}, errors.New("actor_id not found in context")
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

	if err := s.authorized(ctx, old.ID); err != nil {
		return resource.Chunk{}, fmt.Errorf("authorize: %w", err)
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

func (s *svc) UpdateThumbnail(ctx context.Context, chunkID string, imgData []byte) error {
	if err := s.authorized(ctx, chunkID); err != nil {
		return fmt.Errorf("authorize: %w", err)
	}

	cfg, _, err := image.DecodeConfig(bytes.NewBuffer(imgData))
	if err != nil {
		if errors.Is(err, image.ErrFormat) {
			return apierrs.ErrInvalidThumbnailFormat
		}
		return fmt.Errorf("decode config: %w", err)
	}

	if cfg.Width != resource.MaxChunkThumbnailDimensions && cfg.Height != resource.MaxChunkThumbnailDimensions {
		return apierrs.ErrInvalidThumbnailDimensions
	}

	if len(imgData)/1000 > s.cfg.ThumbnailMaxSizeKB {
		return apierrs.ErrInvalidThumbnailSize
	}

	obj := blob.Object{
		Data: nopReadSeekCloser{bytes.NewReader(imgData)},
	}

	h, err := obj.Hash()
	if err != nil {
		return fmt.Errorf("hash: %w", err)
	}

	if err := s.repo.UpdateThumbnail(ctx, chunkID, h); err != nil {
		return fmt.Errorf("db: %w", err)
	}

	if err := s.s3Store.PutBlob(ctx, blob.CASKeyPrefix, []blob.Object{obj}); err != nil {
		return fmt.Errorf("put image: %w", err)
	}

	return nil
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

func (s *svc) authorized(ctx context.Context, chunkID string) error {
	actorID, ok := ctx.Value(contextkey.ActorID).(string)
	if !ok {
		return errors.New("actor_id not found in context")
	}

	if err := s.access.AccessAuthorized(
		ctx,
		authz.WithOwnershipRule(actorID, authz.ChunkResourceDef(chunkID)),
	); err != nil {
		return fmt.Errorf("access: %w", err)
	}

	return nil
}

type nopReadSeekCloser struct {
	io.ReadSeeker
}

func (nopReadSeekCloser) Close() error { return nil }
