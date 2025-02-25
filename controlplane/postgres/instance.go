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

func createInstanceParams(nodeID string, instance chunk.Instance) (instanceParams, error) {
	createdAt := pgtype.Timestamptz{}
	if err := createdAt.Scan(instance.CreatedAt); err != nil {
		return instanceParams{}, fmt.Errorf("scan updated at: %w", err)
	}

	updatedAt := pgtype.Timestamptz{}
	if err := updatedAt.Scan(instance.UpdatedAt); err != nil {
		return instanceParams{}, fmt.Errorf("scan updated at: %w", err)
	}

	return instanceParams{
		create: query.CreateInstanceParams{
			ID:        instance.ID,
			ChunkID:   instance.Chunk.ID,
			FlavorID:  instance.ChunkFlavor.ID,
			NodeID:    nodeID,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
	}, nil
}

func (db *DB) CreateInstance(ctx context.Context, instance chunk.Instance, nodeID string) (chunk.Instance, error) {
	params, err := createInstanceParams(nodeID, instance)
	if err != nil {
		return chunk.Instance{}, fmt.Errorf("instance params: %w", err)
	}

	var ret chunk.Instance
	if err := db.doTX(ctx, func(q *query.Queries) error {
		if err := q.CreateInstance(ctx, params.create); err != nil {
			return fmt.Errorf("create instance: %w", err)
		}

		rows, err := q.GetInstance(ctx, params.create.ID)
		if err != nil {
			return fmt.Errorf("get instance: %w", err)
		}

		// we retrieve multiple rows when we call GetInstance
		// chunk data and instance data will stay the same, what
		// will change is the flavor data. there will be one row
		// for each flavor the chunk has.
		//
		// so it is safe that we use the first row here, because
		// the data will stay the same.
		row := rows[0]

		ret = chunk.Instance{
			ID:        row.ID,
			Address:   row.Address,
			State:     chunk.InstanceState(row.State),
			CreatedAt: row.CreatedAt.Time.UTC(),
			UpdatedAt: row.UpdatedAt.Time.UTC(),
			Chunk: chunk.Chunk{
				ID:          row.ID_3,
				Name:        row.Name_2,
				Description: row.Description,
				Tags:        row.Tags,
				CreatedAt:   row.CreatedAt_3.Time.UTC(),
				UpdatedAt:   row.UpdatedAt_3.Time.UTC(),
			},
		}

		flavors := make([]chunk.Flavor, 0, len(rows))
		for _, instanceRow := range rows {
			f := chunk.Flavor{
				ID:                 instanceRow.ID_2,
				Name:               instanceRow.Name,
				BaseImageURL:       instanceRow.BaseImageUrl,
				CheckpointImageURL: instanceRow.CheckpointImageUrl,
				CreatedAt:          instanceRow.CreatedAt_2.Time.UTC(),
				UpdatedAt:          instanceRow.UpdatedAt_2.Time.UTC(),
			}

			if instanceRow.FlavorID == f.ID {
				ret.ChunkFlavor = f
			}

			flavors = append(flavors, f)
		}

		ret.Chunk.Flavors = flavors

		return nil
	}); err != nil {
		return chunk.Instance{}, err
	}

	return ret, nil
}
