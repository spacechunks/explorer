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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spacechunks/explorer/controlplane/postgres/query"
)

type DB struct {
	pool *pgxpool.Pool
}

func (db *DB) do(ctx context.Context, fn func(q *query.Queries) error) error {
	if err := db.pool.AcquireFunc(ctx, func(conn *pgxpool.Conn) error {
		return fn(query.New(conn))
	}); err != nil {
		return err
	}
	return nil
}
