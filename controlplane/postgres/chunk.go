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
	"github.com/jackc/pgx/v5/pgtype"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/postgres/query"
	"github.com/spacechunks/explorer/controlplane/resource"
	"github.com/spacechunks/explorer/internal/file"
)

func (db *DB) CreateChunk(ctx context.Context, c resource.Chunk) (resource.Chunk, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return resource.Chunk{}, fmt.Errorf("generate id: %w", err)
	}

	params := query.CreateChunkParams{
		ID:          id.String(),
		Name:        c.Name,
		Description: c.Description,
		Tags:        c.Tags,
		OwnerID:     c.Owner.ID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	var ret resource.Chunk
	if err := db.doTX(ctx, func(tx pgx.Tx, q *query.Queries) error {
		if err := q.CreateChunk(ctx, params); err != nil {
			return fmt.Errorf("create chunk: %w", err)
		}

		created, err := db.getChunkByID(ctx, q, id.String())
		if err != nil {
			return fmt.Errorf("get chunk: %w", err)
		}

		ret = created
		return nil
	}); err != nil {
		return resource.Chunk{}, err
	}

	return ret, nil
}

func (db *DB) GetChunkByID(ctx context.Context, id string) (resource.Chunk, error) {
	// FIXME: allow fetching multiple chunks at once
	var ret resource.Chunk
	if err := db.do(ctx, func(q *query.Queries) error {
		c, err := db.getChunkByID(ctx, q, id)
		if err != nil {
			return err
		}
		ret = c
		return nil
	}); err != nil {
		return resource.Chunk{}, err
	}

	return ret, nil
}

func (db *DB) UpdateChunk(ctx context.Context, c resource.Chunk) (resource.Chunk, error) {
	params := query.UpdateChunkParams{
		Name:        c.Name,
		Description: c.Description,
		Tags:        c.Tags,
		ID:          c.ID,
	}

	var ret resource.Chunk
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
		return resource.Chunk{}, err
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

func (db *DB) ListChunks(ctx context.Context) ([]resource.Chunk, error) {
	var ret []resource.Chunk
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
				MinecraftVersion:       r.MinecraftVersion.String,
				Hash:                   r.Hash.String,
				BuildStatus:            string(r.BuildStatus.BuildStatus),
				ChangeHash:             r.ChangeHash.String,
				FilesUploaded:          r.FilesUploaded.Bool,
				FlavorVersionCreatedAt: r.CreatedAt_3.Time,

				FilePath: r.FilePath.String,
				FileHash: r.FileHash.String,

				UserID:        *r.ID_4,
				UserNickname:  r.Nickname.String,
				UserEmail:     r.Email.String,
				UserCreatedAt: r.CreatedAt_5.Time,
				UserUpdatedAt: r.UpdatedAt_3.Time,
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

			m[r.ID] = append(m[r.ID], rel)
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

func (db *DB) SupportedMinecraftVersions(ctx context.Context) ([]string, error) {
	var ret []string
	if err := db.do(ctx, func(q *query.Queries) error {
		versions, err := q.AllMinecraftVersions(ctx)
		if err != nil {
			return err
		}

		ret = versions
		return nil
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (db *DB) MinecraftVersionExists(ctx context.Context, version string) (bool, error) {
	var ret bool
	if err := db.do(ctx, func(q *query.Queries) error {
		exists, err := q.MinecraftVersionExists(ctx, version)
		if err != nil {
			return err
		}

		ret = exists
		return nil
	}); err != nil {
		return false, err
	}
	return ret, nil
}

func (db *DB) UpdateThumbnail(ctx context.Context, chunkID string, imgHash string) error {
	if err := db.do(ctx, func(q *query.Queries) error {
		return q.UpdateChunkThumbnail(ctx, query.UpdateChunkThumbnailParams{
			ID: chunkID,
			ThumbnailHash: pgtype.Text{
				String: imgHash,
				Valid:  true,
			},
		})
	}); err != nil {
		return err
	}

	return nil
}

func (db *DB) getChunkByID(ctx context.Context, q *query.Queries, id string) (resource.Chunk, error) {
	rows, err := q.GetChunkByID(ctx, id)
	if err != nil {
		return resource.Chunk{}, err
	}

	if len(rows) == 0 {
		return resource.Chunk{}, apierrs.ErrChunkNotFound
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
			MinecraftVersion:       r.MinecraftVersion.String,
			Hash:                   r.Hash.String,
			BuildStatus:            string(r.BuildStatus.BuildStatus),
			ChangeHash:             r.ChangeHash.String,
			FilesUploaded:          r.FilesUploaded.Bool,
			FlavorVersionCreatedAt: r.CreatedAt_3.Time,

			FilePath: r.FilePath.String,
			FileHash: r.FileHash.String,

			UserID:        *r.ID_4,
			UserNickname:  r.Nickname.String,
			UserEmail:     r.Email.String,
			UserCreatedAt: r.CreatedAt_5.Time,
			UserUpdatedAt: r.UpdatedAt_3.Time,
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

		relationRows = append(relationRows, rel)
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
	MinecraftVersion       string
	Hash                   string
	BuildStatus            string
	ChangeHash             string
	FilesUploaded          bool
	FlavorVersionCreatedAt time.Time
	PresingedURLExpiryDate *time.Time
	PresignedURL           *string

	FilePath string
	FileHash string

	UserID        string
	UserNickname  string
	UserEmail     string
	UserCreatedAt time.Time
	UserUpdatedAt time.Time
}

func collectChunks(rows []chunkRelationsRow) resource.Chunk {
	// crazy shit code ahead, but really at this point
	// i. couldn't. care. less. ill fix this later, for
	// now it works.

	var (
		ret                resource.Chunk
		flavorMap          = make(map[string]resource.Flavor)
		versionMap         = make(map[string]resource.FlavorVersion)
		fhMap              = make(map[string][]file.Hash)
		versionToFlavorMap = make(map[string]string)

		row = rows[0]
		c   = resource.Chunk{
			ID:          row.ChunkID,
			Name:        row.ChunkName,
			Description: row.Description,
			Tags:        row.Tags,
			CreatedAt:   row.ChunkCreatedAt.UTC(),
			UpdatedAt:   row.ChunkUpdatedAt.UTC(),
			Owner: resource.User{
				ID:        row.UserID,
				Nickname:  row.UserNickname,
				Email:     row.UserEmail,
				CreatedAt: row.UserCreatedAt,
				UpdatedAt: row.UserUpdatedAt,
			},
		}
	)

	for _, r := range rows {
		if r.FlavorID != nil {
			_, ok := flavorMap[*r.FlavorID]
			if !ok {
				flavorMap[*r.FlavorID] = resource.Flavor{
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
				versionMap[*r.FlavorVersionID] = resource.FlavorVersion{
					ID:                     *r.FlavorVersionID,
					Version:                r.Version,
					MinecraftVersion:       r.MinecraftVersion,
					Hash:                   r.Hash,
					BuildStatus:            resource.FlavorVersionBuildStatus(r.BuildStatus),
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
			return f.Versions[i].CreatedAt.After(f.Versions[j].CreatedAt)
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
