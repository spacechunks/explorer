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

package postgres

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/spacechunks/explorer/controlplane/chunk"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/postgres/query"
	"github.com/spacechunks/explorer/internal/file"
)

func (db *DB) CreateChunk(ctx context.Context, c chunk.Chunk) (chunk.Chunk, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return chunk.Chunk{}, fmt.Errorf("generate id: %w", err)
	}

	params := query.CreateChunkParams{
		ID:          id.String(),
		Name:        c.Name,
		Description: c.Description,
		Tags:        c.Tags,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := db.do(ctx, func(q *query.Queries) error {
		if err := q.CreateChunk(ctx, params); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return chunk.Chunk{}, err
	}

	c.ID = id.String()
	c.CreatedAt = params.CreatedAt
	c.UpdatedAt = params.UpdatedAt
	return c, nil
}

func (db *DB) GetChunkByID(ctx context.Context, id string) (chunk.Chunk, error) {
	// FIXME: allow fetching multiple chunks at once
	var ret chunk.Chunk
	if err := db.do(ctx, func(q *query.Queries) error {
		c, err := db.getChunkByID(ctx, q, id)
		if err != nil {
			return err
		}
		ret = c
		return nil
	}); err != nil {
		return chunk.Chunk{}, err
	}

	return ret, nil
}

func (db *DB) UpdateChunk(ctx context.Context, c chunk.Chunk) (chunk.Chunk, error) {
	params := query.UpdateChunkParams{
		Name:        c.Name,
		Description: c.Description,
		Tags:        c.Tags,
		ID:          c.ID,
	}

	var ret chunk.Chunk
	if err := db.doTX(ctx, func(tx pgx.Tx, q *query.Queries) error {
		if err := q.UpdateChunk(ctx, params); err != nil {
			return fmt.Errorf("update chunk: %w", err)
		}

		c, err := db.getChunkByID(ctx, q, params.ID)
		if err != nil {
			return fmt.Errorf("get chunk: %w", err)
		}

		ret = c
		return nil
	}); err != nil {
		return chunk.Chunk{}, err
	}

	return ret, nil
}

func (db *DB) ChunkExists(ctx context.Context, id string) (bool, error) {
	var ret bool
	if err := db.do(ctx, func(q *query.Queries) error {
		ok, err := q.ChunkExists(ctx, id)
		if err != nil {
			return err
		}

		ret = ok
		return nil
	}); err != nil {
		return false, err
	}

	return ret, nil
}

func (db *DB) ListChunks(ctx context.Context) ([]chunk.Chunk, error) {
	var ret []chunk.Chunk
	if err := db.do(ctx, func(q *query.Queries) error {
		rows, err := q.ListChunks(ctx)
		if err != nil {
			return err
		}

		m := make(map[string][]chunkRelationsRow)
		for _, r := range rows {
			rel := chunkRelationsRow{
				ChunkID:        r.ID,
				ChunkName:      r.Name,
				Description:    r.Description,
				Tags:           r.Tags,
				ChunkCreatedAt: r.CreatedAt.UTC(),
				ChunkUpdatedAt: r.UpdatedAt.UTC(),

				FlavorID:        r.ID_2,
				FlavorName:      r.Name_2.String,
				FlavorCreatedAt: r.CreatedAt_2.Time,
				FlavorUpdatedAt: r.UpdatedAt_2.Time,

				FlavorVersionID:        r.ID_3,
				FlavorVersionFlavorID:  r.FlavorID,
				Version:                r.Version.String,
				Hash:                   r.Hash.String,
				BuildStatus:            string(r.BuildStatus.BuildStatus),
				ChangeHash:             r.ChangeHash.String,
				FilesUploaded:          r.FilesUploaded.Bool,
				FlavorVersionCreatedAt: r.CreatedAt_3.Time,

				FilePath: r.FilePath.String,
				FileHash: r.FileHash.String,
			}

			// sqlc is not able to generate a *time.Time from nullable timestamptz
			var expiryDate *time.Time
			if r.PresignedUrlExpiryDate.Valid {
				expiryDate = &r.PresignedUrlExpiryDate.Time
			}

			var presignedURL *string
			if r.PresignedUrl.Valid {
				presignedURL = &r.PresignedUrl.String
			}

			rel.PresingedURLExpiryDate = expiryDate
			rel.PresignedURL = presignedURL

			m[r.ID] = append(m[r.ID])
		}

		for _, rows := range m {
			ret = append(ret, collectChunks(rows))
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (db *DB) getChunkByID(ctx context.Context, q *query.Queries, id string) (chunk.Chunk, error) {
	rows, err := q.GetChunkByID(ctx, id)
	if err != nil {
		return chunk.Chunk{}, err
	}

	if len(rows) == 0 {
		return chunk.Chunk{}, apierrs.ErrChunkNotFound
	}

	relationRows := make([]chunkRelationsRow, 0, len(rows))

	for _, r := range rows {
		rel := chunkRelationsRow{
			ChunkID:        r.ID,
			ChunkName:      r.Name,
			Description:    r.Description,
			Tags:           r.Tags,
			ChunkCreatedAt: r.CreatedAt.UTC(),
			ChunkUpdatedAt: r.UpdatedAt.UTC(),

			FlavorID:        r.ID_2,
			FlavorName:      r.Name_2.String,
			FlavorCreatedAt: r.CreatedAt_2.Time,
			FlavorUpdatedAt: r.UpdatedAt_2.Time,

			FlavorVersionID:        r.ID_3,
			FlavorVersionFlavorID:  r.FlavorID,
			Version:                r.Version.String,
			Hash:                   r.Hash.String,
			BuildStatus:            string(r.BuildStatus.BuildStatus),
			ChangeHash:             r.ChangeHash.String,
			FilesUploaded:          r.FilesUploaded.Bool,
			FlavorVersionCreatedAt: r.CreatedAt_3.Time,

			FilePath: r.FilePath.String,
			FileHash: r.FileHash.String,
		}

		// sqlc is not able to generate a *time.Time from nullable timestamptz
		var expiryDate *time.Time
		if r.PresignedUrlExpiryDate.Valid {
			expiryDate = &r.PresignedUrlExpiryDate.Time
		}

		var presignedURL *string
		if r.PresignedUrl.Valid {
			presignedURL = &r.PresignedUrl.String
		}

		rel.PresingedURLExpiryDate = expiryDate
		rel.PresignedURL = presignedURL

		relationRows = append(relationRows)
	}

	return collectChunks(relationRows), nil
}

type chunkRelationsRow struct {
	ChunkID        string
	ChunkName      string
	Description    string
	Tags           []string
	ChunkCreatedAt time.Time
	ChunkUpdatedAt time.Time

	FlavorID        *string
	FlavorName      string
	FlavorCreatedAt time.Time
	FlavorUpdatedAt time.Time

	FlavorVersionID        *string
	FlavorVersionFlavorID  *string
	Version                string
	Hash                   string
	BuildStatus            string
	ChangeHash             string
	FilesUploaded          bool
	FlavorVersionCreatedAt time.Time
	PresingedURLExpiryDate *time.Time
	PresignedURL           *string

	FilePath string
	FileHash string
}

func collectChunks(rows []chunkRelationsRow) chunk.Chunk {
	// crazy shit code ahead, but really at this point
	// i. couldn't. care. less. ill fix this later, for
	// now it works.

	var (
		ret                chunk.Chunk
		flavorMap          = make(map[string]chunk.Flavor)
		versionMap         = make(map[string]chunk.FlavorVersion)
		fhMap              = make(map[string][]file.Hash)
		versionToFlavorMap = make(map[string]string)

		row = rows[0]
		c   = chunk.Chunk{
			ID:          row.ChunkID,
			Name:        row.ChunkName,
			Description: row.Description,
			Tags:        row.Tags,
			CreatedAt:   row.ChunkCreatedAt.UTC(),
			UpdatedAt:   row.ChunkUpdatedAt.UTC(),
		}
	)

	for _, r := range rows {
		if r.FlavorID != nil {
			_, ok := flavorMap[*r.FlavorID]
			if !ok {
				flavorMap[*r.FlavorID] = chunk.Flavor{
					ID:        *r.FlavorID,
					Name:      r.FlavorName,
					CreatedAt: r.FlavorCreatedAt,
					UpdatedAt: r.FlavorUpdatedAt,
				}
			}
		}

		if r.FlavorVersionID != nil {
			_, ok := versionMap[*r.FlavorVersionID]
			if !ok {
				versionToFlavorMap[*r.FlavorVersionID] = *r.FlavorID
				versionMap[*r.FlavorVersionID] = chunk.FlavorVersion{
					ID:                     *r.FlavorVersionID,
					Version:                r.Version,
					Hash:                   r.Hash,
					BuildStatus:            chunk.BuildStatus(r.BuildStatus),
					ChangeHash:             r.ChangeHash,
					FilesUploaded:          r.FilesUploaded,
					CreatedAt:              r.FlavorVersionCreatedAt,
					PresignedURLExpiryDate: r.PresingedURLExpiryDate,
					PresignedURL:           r.PresignedURL,
				}
			}
		}

		if r.FlavorVersionID != nil {
			contains := slices.ContainsFunc(fhMap[*r.FlavorVersionID], func(fh file.Hash) bool {
				return fh.Path == r.FilePath
			})

			if !contains {
				fhMap[*r.FlavorVersionID] = append(fhMap[*r.FlavorVersionID], file.Hash{
					Path: r.FilePath,
					Hash: r.FileHash,
				})
			}
		}
	}

	for k, hashes := range fhMap {
		ver := versionMap[k]
		ver.FileHashes = hashes
		versionMap[k] = ver
	}

	for _, v := range versionMap {
		flavorID := versionToFlavorMap[v.ID]
		flavor := flavorMap[flavorID]
		flavor.Versions = append(flavor.Versions, v)
		flavorMap[flavorID] = flavor
	}

	for _, f := range flavorMap {
		sort.Slice(f.Versions, func(i, j int) bool {
			// the latest flavor version will be the first entry in the slice
			return f.Versions[i].CreatedAt.Before(f.Versions[j].CreatedAt)
		})
		for _, v := range f.Versions {
			sort.Slice(v.FileHashes, func(i, j int) bool {
				return strings.Compare(v.FileHashes[i].Path, v.FileHashes[j].Path) < 0
			})
		}
	}

	c.Flavors = slices.Collect(maps.Values(flavorMap))
	sort.Slice(c.Flavors, func(i, j int) bool {
		// the latest flavor will be the first entry in the slice
		return c.Flavors[i].CreatedAt.Before(c.Flavors[j].CreatedAt)
	})

	ret = c

	return ret
}
