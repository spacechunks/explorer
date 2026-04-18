package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spacechunks/explorer/controlplane/resource"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchiveChunk(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
		c   = fixture.Chunk(func(tmp *resource.Chunk) {
			tmp.Flavors = nil
		})
	)

	pg.Run(t, ctx)
	pg.InsertMinecraftVersion(t)
	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	err := pg.DB.ArchiveChunk(ctx, c)
	require.NoError(t, err)

	r := pg.Pool.QueryRow(ctx, `SELECT id, data, owner_id FROM chunk_archive WHERE id = $1`, c.ID)

	var (
		id      string
		ownerID string
		data    []byte
	)

	err = r.Scan(&id, &data, &ownerID)
	require.NoError(t, err)

	assert.Equal(t, c.ID, id)
	assert.Equal(t, c.Owner.ID, ownerID)

	expected, err := json.MarshalIndent(c, "", "  ")
	require.NoError(t, err)

	var tmp resource.Chunk

	// we have unmarshal and marshal again, because the keys of the json objects are not sorted
	// when reading from the db. marshaling again will have them in the same order as "expected",
	// thus not failing the test.
	err = json.Unmarshal(data, &tmp)
	require.NoError(t, err)

	actual, err := json.MarshalIndent(c, "", "  ")
	require.NoError(t, err)

	if d := cmp.Diff(string(expected), string(actual)); d != "" {
		t.Fatalf("mismatch (-want +got):\n%s", d)
	}

	err = pg.Pool.QueryRow(ctx, `SELECT id FROM chunks WHERE id = $1`, c.ID).Scan()
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestArchiveFlavor(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
		c   = fixture.Chunk()
	)

	pg.Run(t, ctx)
	pg.InsertMinecraftVersion(t)
	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	f := c.Flavors[0]

	err := pg.DB.ArchiveFlavor(ctx, c.ID, f)
	require.NoError(t, err)

	r := pg.Pool.QueryRow(ctx, `SELECT id, chunk_id, data FROM flavor_archive WHERE id = $1`, f.ID)

	var (
		id      string
		chunkID string
		data    []byte
	)

	err = r.Scan(&id, &chunkID, &data)
	require.NoError(t, err)

	assert.Equal(t, f.ID, id)
	assert.Equal(t, c.ID, chunkID)

	expected, err := json.MarshalIndent(f, "", "  ")
	require.NoError(t, err)

	var tmp resource.Flavor

	// we have unmarshal and marshal again, because the keys of the json objects are not sorted
	// when reading from the db. marshaling again will have them in the same order as "expected",
	// thus not failing the test.
	err = json.Unmarshal(data, &tmp)
	require.NoError(t, err)

	actual, err := json.MarshalIndent(f, "", "  ")
	require.NoError(t, err)

	if d := cmp.Diff(string(expected), string(actual)); d != "" {
		t.Fatalf("mismatch (-want +got):\n%s", d)
	}

	err = pg.Pool.QueryRow(ctx, `SELECT id FROM flavors WHERE id = $1`, f.ID).Scan()
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestArchiveFlavorVersion(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
		c   = fixture.Chunk()
	)

	pg.Run(t, ctx)
	pg.InsertMinecraftVersion(t)
	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	f := c.Flavors[0]
	v := f.Versions[0]

	err := pg.DB.ArchiveFlavorVersion(ctx, f.ID, v)
	require.NoError(t, err)

	r := pg.Pool.QueryRow(ctx, `SELECT id, flavor_id, data FROM flavor_version_archive WHERE id = $1`, v.ID)

	var (
		id       string
		flavorID string
		data     []byte
	)

	err = r.Scan(&id, &flavorID, &data)
	require.NoError(t, err)

	assert.Equal(t, v.ID, id)
	assert.Equal(t, f.ID, flavorID)

	expected, err := json.MarshalIndent(v, "", "  ")
	require.NoError(t, err)

	var tmp resource.FlavorVersion

	// we have unmarshal and marshal again, because the keys of the json objects are not sorted
	// when reading from the db. marshaling again will have them in the same order as "expected",
	// thus not failing the test.
	err = json.Unmarshal(data, &tmp)
	require.NoError(t, err)

	actual, err := json.MarshalIndent(v, "", "  ")
	require.NoError(t, err)

	if d := cmp.Diff(string(expected), string(actual)); d != "" {
		t.Fatalf("mismatch (-want +got):\n%s", d)
	}

	err = pg.Pool.QueryRow(ctx, `SELECT id FROM flavor_versions WHERE id = $1`, v.ID).Scan()
	require.ErrorIs(t, err, sql.ErrNoRows)
}
