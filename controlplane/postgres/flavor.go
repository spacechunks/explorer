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

type flavorParams struct {
	create query.CreateFlavorParams
}

func createFlavorParams(flavor chunk.Flavor, chunkID string) flavorParams {
	return flavorParams{
		create: query.CreateFlavorParams{
			ID:                 flavor.ID,
			ChunkID:            chunkID,
			Name:               flavor.Name,
			BaseImageUrl:       flavor.BaseImageURL,
			CheckpointImageUrl: flavor.CheckpointImageURL,
		},
	}
}

func (db *DB) CreateFlavor(ctx context.Context, flavor chunk.Flavor, chunkID string) (chunk.Flavor, error) {
	params := createFlavorParams(flavor, chunkID)

	var ret chunk.Flavor
	if err := db.do(ctx, func(q *query.Queries) error {
		f, err := q.CreateFlavor(ctx, params.create)
		if err != nil {
			return fmt.Errorf("create flavor: %w", err)
		}

		ret = chunk.Flavor{
			ID:                 f.ID,
			Name:               f.Name,
			BaseImageURL:       f.BaseImageUrl,
			CheckpointImageURL: f.CheckpointImageUrl,
			CreatedAt:          f.CreatedAt.Time,
			UpdatedAt:          f.UpdatedAt.Time,
		}
		return nil
	}); err != nil {
		return chunk.Flavor{}, err
	}

	return ret, nil
}
