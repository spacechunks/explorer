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
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
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
		ctx = context.Background()
		pg  = fixture.NewPostgres()
		c   = fixture.Chunk()
	)

	pg.Run(t, ctx)
	pg.InsertNode(t)

	c.Flavors = []chunk.Flavor{c.Flavors[0]}
	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	expected := fixture.Instance()
	expected.Chunk = c
	expected.ChunkFlavor = c.Flavors[0]
	expected.Port = nil // port will not be saved when creating

	actual, err := pg.DB.CreateInstance(ctx, expected, fixture.Node().ID)
	require.NoError(t, err)

	if d := cmp.Diff(
		expected,
		actual,
		test.IgnoreFields(test.IgnoredInstanceFields...),
		cmpopts.EquateComparable(netip.Addr{}),
	); d != "" {
		t.Fatalf("diff (-want +got):\n%s", d)
	}
}

func TestDBListInstances(t *testing.T) {
	// TODO: at some point test that we are returning an empty array
	//       and not a nil one
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
		c   = fixture.Chunk()
	)

	pg.Run(t, ctx)

	pg.InsertNode(t)

	// make sure we only have one flavor, the fixture has 2 configured by default
	// but for this test we only we need one.
	c.Flavors = []chunk.Flavor{c.Flavors[0]}
	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	expected := []instance.Instance{
		fixture.Instance(func(i *instance.Instance) {
			i.ID = test.NewUUIDv7(t)
			i.Chunk = c
			i.ChunkFlavor = c.Flavors[0]
			i.Port = nil // port will not be saved when creating
		}),
		fixture.Instance(func(i *instance.Instance) {
			i.ID = test.NewUUIDv7(t)
			i.Chunk = c
			i.ChunkFlavor = c.Flavors[0]
			i.Port = nil // port will not be saved when creating
		}),
	}

	for _, i := range expected {
		_, err := pg.DB.CreateInstance(ctx, i, fixture.Node().ID)
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

func TestGetInstanceByID(t *testing.T) {
	tests := []struct {
		name     string
		expected instance.Instance
		create   bool
		err      error
	}{
		{
			name:     "works",
			create:   true,
			expected: fixture.Instance(),
		},
		{
			name:     "not found",
			expected: fixture.Instance(),
			err:      apierrs.ErrInstanceNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				pg  = fixture.NewPostgres()
			)

			pg.Run(t, ctx)
			pg.InsertNode(t)

			if tt.create {
				pg.CreateChunk(t, &tt.expected.Chunk, fixture.CreateOptionsAll)
				tt.expected.ChunkFlavor = tt.expected.Chunk.Flavors[0]

				_, err := pg.DB.CreateInstance(ctx, tt.expected, fixture.Node().ID)
				require.NoError(t, err)

				_, err = pg.Pool.Exec(
					ctx,
					`UPDATE instances SET port = $1 WHERE id = $2`,
					tt.expected.Port,
					tt.expected.ID,
				)
				require.NoError(t, err)
			}

			actual, err := pg.DB.GetInstanceByID(ctx, tt.expected.ID)

			if !tt.create {
				require.ErrorIs(t, err, tt.err)
				return
			}

			require.NoError(t, err)

			if d := cmp.Diff(
				tt.expected,
				actual,
				test.IgnoreFields(test.IgnoredInstanceFields...),
				cmpopts.EquateComparable(netip.Addr{}),
			); d != "" {
				t.Fatalf("diff (-want +got):\n%s", d)
			}
		})
	}
}

func TestGetInstancesByNodeID(t *testing.T) {
	var (
		ctx    = context.Background()
		pg     = fixture.NewPostgres()
		nodeID = fixture.Node().ID
	)

	pg.Run(t, ctx)
	pg.InsertNode(t)

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

	for i := range chunks {
		pg.CreateChunk(t, &chunks[i], fixture.CreateOptionsAll)

		ins := instance.Instance{
			ID:          test.NewUUIDv7(t),
			Chunk:       chunks[i],
			ChunkFlavor: chunks[i].Flavors[0],
			Address:     netip.MustParseAddr(fixture.Node().Addr),
			State:       instance.StatePending,
			CreatedAt:   chunks[i].CreatedAt,
			UpdatedAt:   chunks[i].UpdatedAt,
		}

		// see FIXME in GetInstancesByNodeID
		ins.Chunk.Flavors = nil

		_, err := pg.DB.CreateInstance(ctx, ins, nodeID)
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
