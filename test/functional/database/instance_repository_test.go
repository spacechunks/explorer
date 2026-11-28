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
	"net/netip"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/internal/resource"
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
	pg.InsertMinecraftVersion(t)
	pg.CreateRiverClient(t)

	c.Flavors = []resource.Flavor{c.Flavors[0]}

	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	expected := fixture.Instance()
	expected.Chunk = c
	expected.FlavorVersion = c.Flavors[0].Versions[0]
	expected.Flavor = c.Flavors[0]
	expected.Port = nil                             // port will not be saved when creating
	expected.Address = ""                           // address will not be known until the server api creates the server
	expected.FlavorVersion.FileHashes = nil         // will not be returned atm
	expected.Chunk.Owner = resource.User{}          // will not be returned atm
	expected.Chunk.Thumbnail = resource.Thumbnail{} // will not be returned atm
	expected.Flavor.Versions = nil                  // will not be returned atm
	expected.Owner = c.Owner

	actual, err := pg.DB.CreateInstance(ctx, expected)
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
	)

	pg.Run(t, ctx)

	pg.InsertNode(t)
	pg.InsertMinecraftVersion(t)
	pg.CreateRiverClient(t)

	// make sure we only have one flavor, the fixture has 2 configured by default
	// but for this test we only we need one.
	//c.Flavors = []resource.Flavor{c.Flavors[0]}
	//pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	expected := []resource.Instance{
		fixture.Instance(func(i *resource.Instance) {
			c := fixture.Chunk(func(tmpC *resource.Chunk) {
				tmpC.ID = test.NewUUIDv7(t)
			})
			i.ID = test.NewUUIDv7(t)
			i.Chunk = c
			i.FlavorVersion = c.Flavors[0].Versions[0]
			i.Flavor = c.Flavors[0]
			i.Port = nil                             // port will not be saved when creating
			i.FlavorVersion.FileHashes = nil         // will not be returned atm
			i.Chunk.Owner = resource.User{}          // will not be returned atm
			i.Chunk.Thumbnail = resource.Thumbnail{} // will not be returned atm
			i.Flavor.Versions = nil                  // will not be returned atm
			i.Owner = c.Owner
		}),
		fixture.Instance(func(i *resource.Instance) {
			c := fixture.Chunk(func(tmpC *resource.Chunk) {
				tmpC.ID = test.NewUUIDv7(t)
			})
			i.Chunk = c
			i.ID = test.NewUUIDv7(t)
			i.FlavorVersion = c.Flavors[0].Versions[0]
			i.Flavor = c.Flavors[0]
			i.Port = nil                             // port will not be saved when creating
			i.FlavorVersion.FileHashes = nil         // will not be returned atm
			i.Chunk.Owner = resource.User{}          // will not be returned atm
			i.Chunk.Thumbnail = resource.Thumbnail{} // will not be returned atm
			i.Flavor.Versions = nil                  // will not be returned atm
			i.Owner = c.Owner
		}),
	}

	for idx := range expected {
		pg.CreateInstance(t, fixture.Node().ID, &expected[idx])
	}

	sort.Slice(expected, func(i, j int) bool {
		return strings.Compare(expected[i].ID, expected[j].ID) < 0
	})

	actual, err := pg.DB.ListInstances(ctx, 1, &expected[0].ID)
	require.NoError(t, err)
	require.Len(t, actual, 1)

	if d := cmp.Diff(
		expected[1:2],
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
		expected resource.Instance
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
			pg.InsertMinecraftVersion(t)
			pg.CreateRiverClient(t)

			if tt.create {
				tt.expected.Owner = tt.expected.Chunk.Owner
				tt.expected.Chunk.Owner = resource.User{}          // will not be returned atm
				tt.expected.Chunk.Thumbnail = resource.Thumbnail{} // will not be returned atm
				tt.expected.Flavor.Versions = nil                  // will not be returned atm

				v := tt.expected.Chunk.Flavors[0].Versions[0]
				v.FileHashes = nil // not returned atm

				tt.expected.FlavorVersion = v

				pg.CreateInstance(t, fixture.Node().ID, &tt.expected)

				_, err := pg.Pool.Exec(
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

func TestCountInstancesByFlavorID(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
		c   = fixture.Chunk()
	)

	pg.Run(t, ctx)
	pg.InsertNode(t)
	pg.InsertMinecraftVersion(t)
	pg.CreateRiverClient(t)
	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	version := c.Flavors[0].Versions[0]

	ins1 := fixture.Instance(func(tmp *resource.Instance) {
		tmp.ID = test.NewUUIDv7(t)
		tmp.FlavorVersion = version
		tmp.Owner = c.Owner
	})
	ins2 := fixture.Instance(func(tmp *resource.Instance) {
		tmp.ID = test.NewUUIDv7(t)
		tmp.FlavorVersion = version
		tmp.Owner = c.Owner
	})

	_, err := pg.DB.CreateInstance(ctx, ins1)
	require.NoError(t, err)

	_, err = pg.DB.CreateInstance(ctx, ins2)
	require.NoError(t, err)

	count, err := pg.DB.CountInstancesByFlavorVersionID(ctx, version.ID)
	require.NoError(t, err)

	require.Equal(t, uint(2), count)
}

func TestUpdateInstance(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
		c   = fixture.Chunk()
	)

	pg.Run(t, ctx)
	pg.InsertNode(t)
	pg.InsertMinecraftVersion(t)
	pg.CreateRiverClient(t)
	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	ins := fixture.Instance(func(tmp *resource.Instance) {
		tmp.ID = test.NewUUIDv7(t)
		tmp.FlavorVersion = c.Flavors[0].Versions[0]
		tmp.Owner = c.Owner
	})

	created, err := pg.DB.CreateInstance(ctx, ins)
	require.NoError(t, err)

	var (
		address = "203.0.113.42"
		port    = uint16(30000)
	)

	require.NoError(t, pg.DB.UpdateInstance(ctx, created.ID, address, port))

	actual, err := pg.DB.GetInstanceByID(ctx, created.ID)
	require.NoError(t, err)

	require.Equal(t, address, actual.Address)
	require.NotNil(t, actual.Port)
	require.Equal(t, port, *actual.Port)
}

func TestUpdateInstanceNoopIfInstanceDoesNotExist(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
	)

	pg.Run(t, ctx)
	pg.InsertNode(t)
	pg.InsertMinecraftVersion(t)
	pg.CreateRiverClient(t)

	err := pg.DB.UpdateInstance(ctx, test.NewUUIDv7(t), "203.0.113.42", 30000)
	require.NoError(t, err)
}

// TODO: add test for applystatusreports
