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

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/postgres/query"
)

type instanceParams struct {
	create query.CreateInstanceParams
}

func createInstanceParams(instance chunk.Instance) (instanceParams, error) {
	createdAt := pgtype.Timestamp{}
	if err := createdAt.Scan(instance.CreatedAt); err != nil {
		return instanceParams{}, fmt.Errorf("scan updated at: %w", err)
	}

	updatedAt := pgtype.Timestamp{}
	if err := updatedAt.Scan(instance.UpdatedAt); err != nil {
		return instanceParams{}, fmt.Errorf("scan updated at: %w", err)
	}

	return instanceParams{
		create: query.CreateInstanceParams{
			ID:      instance.ID,
			ChunkID: instance.Chunk.ID,
			Image:   instance.Image,
			NodeID:  instance.NodeID,
		},
	}, nil
}

func (db *DB) CreateInstance(ctx context.Context, instance chunk.Instance) (chunk.Instance, error) {
	params, err := createInstanceParams(instance)
	if err != nil {
		return chunk.Instance{}, fmt.Errorf("instance params: %w", err)
	}

	var ret chunk.Instance
	if err := db.doTX(ctx, func(q *query.Queries) error {
		if err := q.CreateInstance(ctx, params.create); err != nil {
			return fmt.Errorf("create instance: %w", err)
		}

		row, err := q.GetInstance(ctx, params.create.ID)
		if err != nil {
			return fmt.Errorf("get instance: %w", err)
		}

		ret = chunk.Instance{
			ID: row.ID,
			Chunk: chunk.Chunk{
				ID:          row.ID_2,
				Name:        row.Name,
				Description: row.Description,
				Tags:        row.Tags,
				CreatedAt:   row.CreatedAt_2.Time,
				UpdatedAt:   row.UpdatedAt_2.Time,
			},
			Image:     row.Image,
			NodeID:    row.NodeID,
			Address:   row.Address,
			State:     string(row.State),
			CreatedAt: row.CreatedAt.Time,
			UpdatedAt: row.UpdatedAt.Time,
		}

		return nil
	}); err != nil {
		return chunk.Instance{}, err
	}

	return ret, nil
}
