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

package functional

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertest"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/spacechunks/explorer/test"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FIXME: at some point we need to make sure in the tests
//        that we actually pick the right flavors that belong
//        to a chunk. same goes for flavor and flavor versions
//        currently, we omit checking ids, because they are
//        dynamically generated and it's a bit of a pain to test.

func TestCreateChunk(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
	)

	pg.Run(t, ctx)

	expected := fixture.Chunk()

	c, err := pg.DB.CreateChunk(ctx, expected)
	require.NoError(t, err)

	// ignore id, since it is dynamically generated
	assert.Equal(t, expected.Name, c.Name)
	assert.Equal(t, expected.Description, c.Description)
	assert.Equal(t, expected.Tags, c.Tags)
	assert.NotEmpty(t, c.CreatedAt)
	assert.NotEmpty(t, c.UpdatedAt)
}

func TestGetChunkByID(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
	)
	pg.Run(t, ctx)

	expected := fixture.Chunk()

	pg.CreateChunk(t, &expected, fixture.CreateOptionsAll)

	actual, err := pg.DB.GetChunkByID(ctx, expected.ID)
	require.NoError(t, err)

	if d := cmp.Diff(expected, actual, test.IgnoreFields(test.IgnoredChunkFields...)); d != "" {
		t.Errorf("chunk mismatch (-want +got):\n%s", d)
	}
}

func TestInsertJob(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
	)
	pg.Run(t, ctx)
	pg.CreateRiverClient(t)

	c := fixture.Chunk()
	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	riverJob := job.CreateImage{
		FlavorVersionID: "adawdddd",
		BaseImage:       "111",
		OCIRegistry:     "3333",
	}

	err := pg.DB.InsertJob(ctx, c.Flavors[0].Versions[0].ID, string(chunk.BuildStatusBuildImage), riverJob)
	require.NoError(t, err)

	version, err := pg.DB.FlavorVersionByID(ctx, c.Flavors[0].Versions[0].ID)
	require.NoError(t, err)

	require.Equal(t, chunk.BuildStatusBuildImage, version.BuildStatus)

	rivertest.RequireInserted[*riverpgxv5.Driver, pgx.Tx, job.CreateImage](
		ctx,
		t,
		riverpgxv5.New(pg.Pool),
		riverJob,
		nil,
	)
}
