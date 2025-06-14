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
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/spacechunks/explorer/controlplane/postgres/query"
)

var ErrNotFound = errors.New("not found")

type DB struct {
	logger      *slog.Logger
	pool        *pgxpool.Pool
	riverClient *river.Client[pgx.Tx]
}

type bulkExecer interface {
	Exec(f func(int, error))
	Close() error
}

func NewDB(logger *slog.Logger, pool *pgxpool.Pool) *DB {
	return &DB{
		logger: logger,
		pool:   pool,
	}
}

// SetRiverClient is a workaround to resolve a chicken-egg problem.
// river workers need to be configured before the river client can
// be created. workers depend on services which use the [postgres.DB]
// object. passing the river client in the [postgres.DB] constructor
// will create a circular dependency. db needs river client -> river
// client needs db.
func (db *DB) SetRiverClient(client *river.Client[pgx.Tx]) {
	db.riverClient = client
}

func (db *DB) do(ctx context.Context, fn func(q *query.Queries) error) error {
	if err := db.pool.AcquireFunc(ctx, func(conn *pgxpool.Conn) error {
		return fn(query.New(conn))
	}); err != nil {
		return err
	}
	return nil
}

func (db *DB) doTX(ctx context.Context, fn func(tx pgx.Tx, q *query.Queries) error) error {
	if err := db.pool.AcquireFunc(ctx, func(conn *pgxpool.Conn) error {
		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}

		if err := fn(tx, query.New(tx)); err != nil {
			if txErr := tx.Rollback(ctx); txErr != nil {
				// only log rollback related error, because we want to
				// pass the actual error that caused the rollback back to the caller
				db.logger.ErrorContext(ctx, "failed to rollback tx", "err", txErr)
			}
			return err
		}

		return tx.Commit(ctx)
	}); err != nil {
		return err
	}

	return nil
}

func (db *DB) bulkExecAndClose(execer bulkExecer) error {
	var err error
	execer.Exec(func(i int, e error) {
		err = e
	})

	if err := execer.Close(); err != nil {
		return fmt.Errorf("close execer: %w", err)
	}

	return err
}
