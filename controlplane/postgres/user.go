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
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/postgres/query"
	"github.com/spacechunks/explorer/controlplane/user"
)

func (db *DB) GetUserByEmail(ctx context.Context, email string) (user.User, error) {
	var ret user.User
	if err := db.do(ctx, func(q *query.Queries) error {
		u, err := q.UserByEmail(ctx, email)
		if errors.Is(err, pgx.ErrNoRows) {
			return apierrs.ErrNotFound
		}

		if err != nil {
			return err
		}

		ret = user.User{
			ID:        u.ID,
			Nickname:  u.Nickname,
			Email:     u.Email,
			CreatedAt: u.CreatedAt,
			UpdatedAt: u.UpdatedAt,
		}

		return nil
	}); err != nil {
		return user.User{}, err
	}

	return ret, nil
}
func (db *DB) CreateUser(ctx context.Context, u user.User) (user.User, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return user.User{}, fmt.Errorf("user id: %w", err)
	}

	now := time.Now()

	err = db.do(ctx, func(q *query.Queries) error {
		return q.CreateUser(ctx, query.CreateUserParams{
			ID:        id.String(),
			Nickname:  u.Nickname,
			Email:     u.Email,
			CreatedAt: now,
			UpdatedAt: now,
		})
	})

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		fmt.Println(err)
		if pgErr.Code == "23505" {
			return user.User{}, apierrs.ErrAlreadyExists
		}
	}

	if err != nil {
		return user.User{}, fmt.Errorf("create user: %w", err)
	}

	u.ID = id.String()
	u.CreatedAt = now
	u.UpdatedAt = now
	return u, nil
}
