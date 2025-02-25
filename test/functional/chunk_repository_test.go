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
	"net/netip"
	"sort"
	"strings"
	"testing"

	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/test"
	"github.com/spacechunks/explorer/test/functional/fixture"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateChunk(t *testing.T) {
	var (
		ctx   = context.Background()
		_, db = fixture.RunDB(t)
	)

	expected := fixture.Chunk()

	c, err := db.CreateChunk(ctx, expected)
	require.NoError(t, err)

	assert.Equal(t, expected.ID, c.ID)
	assert.Equal(t, expected.Name, c.Name)
	assert.Equal(t, expected.Description, c.Description)
	assert.Equal(t, expected.Tags, c.Tags)
	assert.NotEmpty(t, c.CreatedAt)
	assert.NotEmpty(t, c.UpdatedAt)
}

func TestCreateInstance(t *testing.T) {
	// TODO:
	// * check that creating does not work if no flavor is present
	// * check that creating does not work if no chunk is present
	// * check that creating does not work if no node is present

	var (
		ctx      = context.Background()
		pool, db = fixture.RunDB(t)
		nodeID   = test.NewUUIDv7(t)
		c        = fixture.Chunk()
	)

	_, err := pool.Exec(ctx, `INSERT INTO nodes (id, address) VALUES ($1, $2)`, nodeID, "198.51.100.1")
	require.NoError(t, err)

	_, err = db.CreateChunk(ctx, c)
	require.NoError(t, err)

	for _, f := range c.Flavors {
		_, err = db.CreateFlavor(ctx, f, c.ID)
		require.NoError(t, err)
	}

	// ^ above are prerequisites

	expected := fixture.Instance

	actual, err := db.CreateInstance(ctx, expected, nodeID)
	require.NoError(t, err)

	assert.Equal(t, expected, actual)
}

func TestGetInstancesByNodeID(t *testing.T) {
	var (
		ctx      = context.Background()
		pool, db = fixture.RunDB(t)
		nodeID   = test.NewUUIDv7(t)
	)

	_, err := pool.Exec(ctx, `INSERT INTO nodes (id, address) VALUES ($1, $2)`, nodeID, "198.51.100.1")
	require.NoError(t, err)

	chunks := []chunk.Chunk{
		fixture.Chunk(func(c *chunk.Chunk) {
			c.ID = "01953e54-8ac5-7c1a-b468-dffdc26d2087"
			c.Name = "chunk1"
			c.Flavors = []chunk.Flavor{
				fixture.Flavor(func(f *chunk.Flavor) {
					f.ID = test.NewUUIDv7(t)
					f.Name = "flavor_" + test.RandHexStr(t)
				}),
			}
		}),
		fixture.Chunk(func(c *chunk.Chunk) {
			c.ID = "01953e54-b686-764a-874f-dbc45b67152c"
			c.Name = "chunk2"
			c.Flavors = []chunk.Flavor{
				fixture.Flavor(func(f *chunk.Flavor) {
					f.ID = test.NewUUIDv7(t)
					f.Name = "flavor_" + test.RandHexStr(t)
				}),
			}
		}),
	}

	var expected []chunk.Instance

	for _, c := range chunks {
		_, err = db.CreateChunk(ctx, c)
		require.NoError(t, err)

		_, err = db.CreateFlavor(ctx, c.Flavors[0], c.ID)
		require.NoError(t, err)

		ins := chunk.Instance{
			ID:          test.NewUUIDv7(t),
			Chunk:       c,
			ChunkFlavor: c.Flavors[0],
			Address:     netip.MustParseAddr("198.51.100.1"),
			State:       chunk.InstanceStatePending,
			CreatedAt:   c.CreatedAt,
			UpdatedAt:   c.UpdatedAt,
		}

		// see FIXME in GetInstancesByNodeID
		ins.Chunk.Flavors = nil

		_, err := db.CreateInstance(ctx, ins, nodeID)
		require.NoError(t, err)

		expected = append(expected, ins)
	}

	sort.Slice(expected, func(i, j int) bool {
		return strings.Compare(expected[i].ID, expected[j].ID) < 0
	})

	acutalInstances, err := db.GetInstancesByNodeID(ctx, nodeID)
	require.NoError(t, err)

	sort.Slice(acutalInstances, func(i, j int) bool {
		return strings.Compare(acutalInstances[i].ID, acutalInstances[j].ID) < 0
	})

	require.Equal(t, expected, acutalInstances)
}
