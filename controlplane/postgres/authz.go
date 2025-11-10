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

	"github.com/spacechunks/explorer/controlplane/postgres/query"
	"github.com/spacechunks/explorer/controlplane/resource"
)

func (db *DB) ChunkOwner(ctx context.Context, chunkID string) (resource.User, error) {
	return getOwner(ctx, db, func(ctx context.Context, q *query.Queries) (query.User, error) {
		return q.ChunkOwnerByChunkID(ctx, chunkID)
	})
}

func (db *DB) FlavorOwner(ctx context.Context, flavorID string) (resource.User, error) {
	return getOwner(ctx, db, func(ctx context.Context, q *query.Queries) (query.User, error) {
		return q.ChunkOwnerByFlavorID(ctx, flavorID)
	})
}

func (db *DB) FlavorVersionOwner(ctx context.Context, flavorVersionID string) (resource.User, error) {
	return getOwner(ctx, db, func(ctx context.Context, q *query.Queries) (query.User, error) {
		return q.ChunkOwnerByFlavorVersionID(ctx, flavorVersionID)
	})
}

func getOwner(
	ctx context.Context,
	db *DB,
	fn func(ctx context.Context, q *query.Queries) (query.User, error),
) (resource.User, error) {
	var ret resource.User
	if err := db.do(ctx, func(q *query.Queries) error {
		u, err := fn(ctx, q)
		if err != nil {
			return err
		}
		ret = resource.User{
			ID:        u.ID,
			Nickname:  u.Nickname,
			Email:     u.Email,
			CreatedAt: u.CreatedAt,
			UpdatedAt: u.UpdatedAt,
		}
		return nil
	}); err != nil {
		return resource.User{}, nil
	}
	return ret, nil
}
