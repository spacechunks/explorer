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
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/stretchr/testify/require"
)

func TestChunkOwner(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
		c   = fixture.Chunk()
	)

	pg.Run(t, ctx)
	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	actual, err := pg.DB.ChunkOwner(ctx, c.ID)
	require.NoError(t, err)

	if d := cmp.Diff(c.Owner, actual); d != "" {
		t.Fatalf("mismatch (-want +got):\n%s", d)
	}
}

func TestFlavorOwner(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
		c   = fixture.Chunk()
	)

	pg.Run(t, ctx)
	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	actual, err := pg.DB.FlavorOwner(ctx, c.Flavors[0].ID)
	require.NoError(t, err)

	if d := cmp.Diff(c.Owner, actual); d != "" {
		t.Fatalf("mismatch (-want +got):\n%s", d)
	}
}

func TestFlavorVersionOwner(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
		c   = fixture.Chunk()
	)

	pg.Run(t, ctx)
	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	actual, err := pg.DB.FlavorVersionOwner(ctx, c.Flavors[0].Versions[0].ID)
	require.NoError(t, err)

	if d := cmp.Diff(c.Owner, actual); d != "" {
		t.Fatalf("mismatch (-want +got):\n%s", d)
	}
}
