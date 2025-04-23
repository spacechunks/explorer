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
	"time"

	"github.com/google/uuid"
	"github.com/spacechunks/explorer/controlplane/chunk"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/postgres/query"
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
		rows, err := q.GetChunkByID(ctx, id)
		if err != nil {
			return err
		}

		if len(rows) == 0 {
			return apierrs.ErrChunkNotFound
		}

		var (
			row     = rows[0]
			flavors = make([]chunk.Flavor, 0, len(rows))
			c       = chunk.Chunk{
				ID:          row.ID,
				Name:        row.Name,
				Description: row.Description,
				Tags:        row.Tags,
				CreatedAt:   row.CreatedAt.UTC(),
				UpdatedAt:   row.UpdatedAt.UTC(),
			}
		)

		for _, r := range rows {
			flavors = append(flavors, chunk.Flavor{
				ID:        r.ID_2,
				Name:      r.Name_2,
				CreatedAt: r.CreatedAt_2.UTC(),
				UpdatedAt: r.UpdatedAt_2.UTC(),
			})
		}

		c.Flavors = flavors
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
	if err := db.doTX(ctx, func(q *query.Queries) error {
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

		m := make(map[string][]query.ListChunksRow)
		for _, r := range rows {
			m[r.ID] = append(m[r.ID], r)
		}

		ret = make([]chunk.Chunk, 0, len(m))

		for _, v := range m {
			var (
				row     = v[0]
				flavors = make([]chunk.Flavor, 0, len(rows))
				c       = chunk.Chunk{
					ID:          row.ID,
					Name:        row.Name,
					Description: row.Description,
					Tags:        row.Tags,
					CreatedAt:   row.CreatedAt.UTC(),
					UpdatedAt:   row.UpdatedAt.UTC(),
				}
			)

			for _, r := range v {
				flavors = append(flavors, chunk.Flavor{
					ID:        r.ID_2,
					Name:      r.Name_2,
					CreatedAt: r.CreatedAt_2.UTC(),
					UpdatedAt: r.UpdatedAt_2.UTC(),
				})
			}

			c.Flavors = flavors
			ret = append(ret, c)
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

	var (
		row     = rows[0]
		flavors = make([]chunk.Flavor, 0, len(rows))
		c       = chunk.Chunk{
			ID:          row.ID,
			Name:        row.Name,
			Description: row.Description,
			Tags:        row.Tags,
			CreatedAt:   row.CreatedAt.UTC(),
			UpdatedAt:   row.UpdatedAt.UTC(),
		}
	)

	for _, r := range rows {
		flavors = append(flavors, chunk.Flavor{
			ID:        r.ID_2,
			Name:      r.Name_2,
			CreatedAt: r.CreatedAt_2.UTC(),
			UpdatedAt: r.UpdatedAt_2.UTC(),
		})
	}

	c.Flavors = flavors
	return c, nil
}
