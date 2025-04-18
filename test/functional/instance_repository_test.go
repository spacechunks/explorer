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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/instance"
	"github.com/spacechunks/explorer/test"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/stretchr/testify/require"
)

func TestCreateInstance(t *testing.T) {
	// TODO:
	// * check that creating does not work if no flavor is present
	// * check that creating does not work if no chunk is present
	// * check that creating does not work if no node is present

	var (
		ctx    = context.Background()
		pg     = fixture.NewPostgres()
		nodeID = test.NewUUIDv7(t)
		c      = fixture.Chunk()
	)

	pg.Run(t, ctx)

	_, err := pg.Pool.Exec(ctx, `INSERT INTO nodes (id, address) VALUES ($1, $2)`, nodeID, "198.51.100.1")
	require.NoError(t, err)

	c.Flavors = []chunk.Flavor{c.Flavors[0]}

	_, err = pg.DB.CreateChunk(ctx, c)
	require.NoError(t, err)

	createdFlavor, err := pg.DB.CreateFlavor(ctx, c.ID, c.Flavors[0])
	require.NoError(t, err)

	// ^ above are prerequisites

	expected := fixture.Instance()
	expected.Chunk = c
	expected.ChunkFlavor = createdFlavor
	expected.Port = nil // port will not be saved when creating

	actual, err := pg.DB.CreateInstance(ctx, expected, nodeID)
	require.NoError(t, err)

	if d := cmp.Diff(
		expected,
		actual,
		test.IgnoreFields(test.IgnoredInstanceFields...),
		cmpopts.EquateComparable(netip.Addr{}),
	); d != "" {
		t.Fatalf("CreateInstance() mismatch (-want +got):\n%s", d)
	}
}

func TestDBListInstances(t *testing.T) {
	// TODO: at some point test that we are returning an empty array
	//       and not a nil one
	var (
		ctx    = context.Background()
		pg     = fixture.NewPostgres()
		nodeID = test.NewUUIDv7(t)
		c      = fixture.Chunk()
	)

	pg.Run(t, ctx)

	_, err := pg.Pool.Exec(ctx, `INSERT INTO nodes (id, address) VALUES ($1, $2)`, nodeID, "198.51.100.1")
	require.NoError(t, err)

	// make sure we only have one flavor, the fixture has 2 configured by default
	// but for this test we only we need one.
	c.Flavors = []chunk.Flavor{c.Flavors[0]}

	_, err = pg.DB.CreateChunk(ctx, c)
	require.NoError(t, err)

	createdFlavor, err := pg.DB.CreateFlavor(ctx, c.ID, c.Flavors[0])
	require.NoError(t, err)

	// ^ above are prerequisites

	expected := []instance.Instance{
		fixture.Instance(func(i *instance.Instance) {
			i.ID = test.NewUUIDv7(t)
			i.Chunk = c
			i.ChunkFlavor = createdFlavor
			i.Port = nil // port will not be saved when creating
		}),
		fixture.Instance(func(i *instance.Instance) {
			i.ID = test.NewUUIDv7(t)
			i.Chunk = c
			i.ChunkFlavor = createdFlavor
			i.Port = nil // port will not be saved when creating
		}),
	}

	for _, i := range expected {
		_, err := pg.DB.CreateInstance(ctx, i, nodeID)
		require.NoError(t, err)
	}

	actual, err := pg.DB.ListInstances(ctx)
	require.NoError(t, err)

	if d := cmp.Diff(
		expected,
		actual,
		test.IgnoreFields(test.IgnoredInstanceFields...),
		cmpopts.EquateComparable(netip.Addr{}),
	); d != "" {
		t.Fatalf("ListInstances() mismatch (-want +got):\n%s", d)
	}
}

func TestGetInstancesByNodeID(t *testing.T) {
	var (
		ctx    = context.Background()
		pg     = fixture.NewPostgres()
		nodeID = test.NewUUIDv7(t)
	)

	pg.Run(t, ctx)

	_, err := pg.Pool.Exec(ctx, `INSERT INTO nodes (id, address) VALUES ($1, $2)`, nodeID, "198.51.100.1")
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

	var expected []instance.Instance

	for _, c := range chunks {
		_, err = pg.DB.CreateChunk(ctx, c)
		require.NoError(t, err)

		createdFlavor, err := pg.DB.CreateFlavor(ctx, c.ID, c.Flavors[0])
		require.NoError(t, err)

		ins := instance.Instance{
			ID:          test.NewUUIDv7(t),
			Chunk:       c,
			ChunkFlavor: createdFlavor,
			Address:     netip.MustParseAddr("198.51.100.1"),
			State:       instance.StatePending,
			CreatedAt:   c.CreatedAt,
			UpdatedAt:   c.UpdatedAt,
		}

		// see FIXME in GetInstancesByNodeID
		ins.Chunk.Flavors = nil

		_, err = pg.DB.CreateInstance(ctx, ins, nodeID)
		require.NoError(t, err)

		expected = append(expected, ins)
	}

	sort.Slice(expected, func(i, j int) bool {
		return strings.Compare(expected[i].ID, expected[j].ID) < 0
	})

	acutalInstances, err := pg.DB.GetInstancesByNodeID(ctx, nodeID)
	require.NoError(t, err)

	sort.Slice(acutalInstances, func(i, j int) bool {
		return strings.Compare(acutalInstances[i].ID, acutalInstances[j].ID) < 0
	})

	if d := cmp.Diff(
		expected,
		acutalInstances,
		test.IgnoreFields(test.IgnoredInstanceFields...),
		cmpopts.EquateComparable(netip.Addr{}),
	); d != "" {
		t.Fatalf("TestGetInstancesByNodeID() mismatch (-want +got):\n%s", d)
	}
}

// TODO: add test for applystatusreports
