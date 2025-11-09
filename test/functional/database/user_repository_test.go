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

package database

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/test"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/stretchr/testify/require"
)

func TestCreateUser(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
	)

	pg.Run(t, ctx)

	expected := fixture.User()

	actual, err := pg.DB.CreateUser(ctx, expected)
	require.NoError(t, err)

	if d := cmp.Diff(expected, actual, test.IgnoreFields(test.IgnoredUserFields...)); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}

func TestGetUserByEmail(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
	)

	pg.Run(t, ctx)

	expected := fixture.User()

	_, err := pg.DB.CreateUser(ctx, expected)
	require.NoError(t, err)

	actual, err := pg.DB.GetUserByEmail(ctx, expected.Email)
	require.NoError(t, err)

	if d := cmp.Diff(expected, actual, test.IgnoreFields(test.IgnoredUserFields...)); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}

func TestCreateUserWithExistingEmailFails(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
		u   = fixture.User()
	)

	pg.Run(t, ctx)

	_, err := pg.DB.CreateUser(ctx, u)
	require.NoError(t, err)

	_, err = pg.DB.CreateUser(ctx, u)
	require.ErrorIs(t, err, apierrs.ErrAlreadyExists)
}
