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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertest"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/spacechunks/explorer/controlplane/resource"
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

	u := fixture.User()

	pg.CreateUser(t, &u)

	expected := fixture.Chunk(func(tmp *resource.Chunk) {
		tmp.Owner = u
	})

	c, err := pg.DB.CreateChunk(ctx, expected)
	require.NoError(t, err)

	// ignore id, since it is dynamically generated
	assert.Equal(t, expected.Name, c.Name)
	assert.Equal(t, expected.Description, c.Description)
	assert.Equal(t, expected.Tags, c.Tags)
	assert.NotEmpty(t, c.CreatedAt)
	assert.NotEmpty(t, c.UpdatedAt)
	assert.Equal(t, "", c.Thumbnail.Hash)
}

func TestGetChunkByID(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
	)

	pg.Run(t, ctx)
	pg.InsertMinecraftVersion(t)

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
	pg.InsertMinecraftVersion(t)
	pg.CreateRiverClient(t)

	c := fixture.Chunk()
	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	riverJob := job.CreateImage{
		FlavorVersionID: "adawdddd",
		BaseImage:       "111",
		OCIRegistry:     "3333",
	}

	err := pg.DB.InsertJob(ctx, c.Flavors[0].Versions[0].ID, string(resource.FlavorVersionBuildStatusBuildImage), riverJob)
	require.NoError(t, err)

	version, err := pg.DB.FlavorVersionByID(ctx, c.Flavors[0].Versions[0].ID)
	require.NoError(t, err)

	require.Equal(t, resource.FlavorVersionBuildStatusBuildImage, version.BuildStatus)

	rivertest.RequireInserted[*riverpgxv5.Driver, pgx.Tx, job.CreateImage](
		ctx,
		t,
		riverpgxv5.New(pg.Pool),
		riverJob,
		nil,
	)
}

func TestUpdateThumbnail(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
	)

	pg.Run(t, ctx)
	pg.InsertMinecraftVersion(t)

	var (
		expectedHash = "some-hash"
		c            = fixture.Chunk()
	)

	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	err := pg.DB.UpdateThumbnail(ctx, c.ID, expectedHash)
	require.NoError(t, err)

	var (
		actualHash = ""
		q          = `SELECT thumbnail_hash from chunks where id = $1`
	)

	err = pg.Pool.QueryRow(ctx, q, c.ID).Scan(&actualHash)
	require.NoError(t, err)

	cmp.Diff(actualHash, expectedHash)
}

func TestGetAllThumbnailHashes(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
		c1  = fixture.Chunk(func(tmp *resource.Chunk) {
			tmp.Thumbnail.Hash = "h1"
		})
		c2 = fixture.Chunk(func(tmp *resource.Chunk) {
			tmp.Thumbnail.Hash = "h2"
		})
		null = fixture.Chunk()
	)

	pg.Run(t, ctx)
	pg.InsertMinecraftVersion(t)

	pg.CreateChunk(t, &c1, fixture.CreateOptionsAll)
	pg.CreateChunk(t, &c2, fixture.CreateOptionsAll)
	pg.CreateChunk(t, &null, fixture.CreateOptionsAll)

	_, err := pg.Pool.Exec(ctx, `UPDATE chunks SET thumbnail_hash = NULL WHERE id = $1`, null.ID)
	require.NoError(t, err)

	expected := map[string]string{
		c1.ID: "h1",
		c2.ID: "h2",
	}

	actual, err := pg.DB.AllChunkThumbnailHashes(ctx)
	require.NoError(t, err)

	if d := cmp.Diff(expected, actual); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}

func TestGetMinecraftVersion(t *testing.T) {
	var (
		ctx      = context.Background()
		pg       = fixture.NewPostgres()
		expected = resource.MinecraftVersion{
			Version:   "1.21.7",
			ImageURL:  "localhost/lol",
			CreatedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		}
	)

	pg.Run(t, ctx)

	_, err := pg.Pool.Exec(
		ctx,
		`INSERT INTO minecraft_versions (version, image_url, created_at) VALUES ($1, $2, $3)`,
		expected.Version,
		expected.ImageURL,
		expected.CreatedAt,
	)
	require.NoError(t, err)

	actual, err := pg.DB.GetMinecraftVersionByVersion(ctx, "1.21.7")
	require.NoError(t, err)

	if d := cmp.Diff(expected, actual); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}

func TestDeleteFlavor(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
		c   = fixture.Chunk()
	)

	pg.Run(t, ctx)
	pg.InsertMinecraftVersion(t)
	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	flavorID := c.Flavors[0].ID

	err := pg.DB.DeleteFlavor(ctx, flavorID)
	require.NoError(t, err)

	var worked int
	err = pg.Pool.
		QueryRow(ctx, `SELECT 1 FROM flavors WHERE id = $1 AND deleted_at IS NOT NULL`, flavorID).
		Scan(&worked)
	require.NoError(t, err)

	require.Equalf(t, 1, worked, "expected deleted at to be not null")
}

func TestGetFlavorByID(t *testing.T) {
	var (
		ctx  = context.Background()
		pg   = fixture.NewPostgres()
		date = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		c    = fixture.Chunk(func(tmp *resource.Chunk) {
			tmp.Flavors[0].DeletedAt = &date
		})
	)

	pg.Run(t, ctx)
	pg.InsertMinecraftVersion(t)
	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	expected := c.Flavors[0]

	actual, err := pg.DB.GetFlavorByID(ctx, expected.ID)
	require.NoError(t, err)

	assert.Equal(t, expected.ID, actual.ID)
	assert.Equal(t, expected.Name, actual.Name)
	assert.NotEmpty(t, actual.CreatedAt)
	assert.NotEmpty(t, actual.UpdatedAt)
	assert.Equal(t, expected.DeletedAt, actual.DeletedAt)
}

func TestListChunksDoesNotReturnDeletedFlavors(t *testing.T) {
	var (
		ctx  = context.Background()
		pg   = fixture.NewPostgres()
		date = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		c    = fixture.Chunk(func(tmp *resource.Chunk) {
			tmp.Flavors[0].DeletedAt = &date
		})
	)

	pg.Run(t, ctx)
	pg.InsertMinecraftVersion(t)

	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	expected := []resource.Chunk{
		{
			ID:          c.ID,
			Name:        c.Name,
			Description: c.Description,
			Tags:        c.Tags,
			Flavors: []resource.Flavor{
				c.Flavors[1],
			},
			Owner:     c.Owner,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
			Thumbnail: c.Thumbnail,
		},
	}

	actual, err := pg.DB.ListChunks(ctx)
	require.NoError(t, err)

	if d := cmp.Diff(expected, actual, test.IgnoreFields(test.IgnoredChunkFields...)); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}

func TestMarkChunkDeleted(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
		c   = fixture.Chunk()
	)

	pg.Run(t, ctx)
	pg.InsertMinecraftVersion(t)
	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	err := pg.DB.MarkChunkAndFlavorsDeleted(ctx, c.ID)
	require.NoError(t, err)

	var worked int
	err = pg.Pool.
		QueryRow(ctx, `SELECT 1 FROM chunks WHERE id = $1 AND deleted_at IS NOT NULL`, c.ID).
		Scan(&worked)
	require.NoError(t, err)

	require.Equalf(t, 1, worked, "expected chunk deleted_at to be not null")

	for _, f := range c.Flavors {
		var tmp int
		err = pg.Pool.
			QueryRow(ctx, `SELECT 1 FROM flavors WHERE id = $1 AND deleted_at IS NOT NULL`, f.ID).
			Scan(&tmp)
		require.NoError(t, err)

		require.Equalf(t, 1, tmp, "expected flavor deleted_at to be not null (%s)", f.Name)
	}
}

func TestGetChunkByIDReturnsErrorIfChunkDeletedAtIsSet(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
	)

	pg.Run(t, ctx)
	pg.InsertMinecraftVersion(t)

	expected := fixture.Chunk(func(tmp *resource.Chunk) {
		tmp.DeletedAt = new(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	})

	pg.CreateChunk(t, &expected, fixture.CreateOptionsAll)

	_, err := pg.DB.GetChunkByID(ctx, expected.ID)
	require.ErrorIs(t, err, apierrs.ErrChunkNotFound)
}

func TestListChunksDoesNotReturnDeletedChunks(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
		c   = fixture.Chunk(func(tmp *resource.Chunk) {
			tmp.DeletedAt = new(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
		})
	)

	pg.Run(t, ctx)
	pg.InsertMinecraftVersion(t)

	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	actual, err := pg.DB.ListChunks(ctx)
	require.NoError(t, err)

	if d := cmp.Diff([]resource.Chunk{}, actual, test.IgnoreFields(test.IgnoredChunkFields...)); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}
