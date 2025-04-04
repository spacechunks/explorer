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

	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/postgres/query"
)

type chunkParams struct {
	create query.CreateChunkParams
	update query.UpdateChunkParams
}

func createChunkParams(c chunk.Chunk) chunkParams {
	return chunkParams{
		create: query.CreateChunkParams{
			ID:          c.ID,
			Name:        c.Name,
			Description: c.Description,
			Tags:        c.Tags,
			CreatedAt:   c.CreatedAt,
			UpdatedAt:   c.UpdatedAt,
		},
		update: query.UpdateChunkParams{
			Name:        c.Name,
			Description: c.Description,
			Tags:        c.Tags,
			ID:          c.ID,
		},
	}
}

func rowToChunk(c query.Chunk) chunk.Chunk {
	return chunk.Chunk{
		ID:          c.ID,
		Name:        c.Name,
		Description: c.Description,
		Tags:        c.Tags,
		CreatedAt:   c.CreatedAt.UTC(),
		UpdatedAt:   c.UpdatedAt.UTC(),
	}
}

func (db *DB) CreateChunk(ctx context.Context, c chunk.Chunk) (chunk.Chunk, error) {
	params := createChunkParams(c)

	var ret chunk.Chunk
	if err := db.do(ctx, func(q *query.Queries) error {
		c, err := q.CreateChunk(ctx, params.create)
		if err != nil {
			return fmt.Errorf("create chunk: %w", err)
		}
		ret = rowToChunk(c)
		return nil
	}); err != nil {
		return chunk.Chunk{}, err
	}

	return ret, nil
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
			return ErrNotFound
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
	params := createChunkParams(c)

	var ret chunk.Chunk
	if err := db.do(ctx, func(q *query.Queries) error {
		c, err := q.UpdateChunk(ctx, params.update)
		if err != nil {
			return fmt.Errorf("update chunk: %w", err)
		}
		ret = rowToChunk(c)
		return nil
	}); err != nil {
		return chunk.Chunk{}, err
	}

	return ret, nil
}
