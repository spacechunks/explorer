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
	"fmt"
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

func TestBestNodeSelectsAndExhaustsSlots(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
	)

	pg.Run(t, ctx)
	pg.InsertNode(t)
	pg.InsertMinecraftVersion(t)

	c := fixture.Chunk()
	c.Flavors = []resource.Flavor{c.Flavors[0]}
	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	n, err := pg.DB.BestNode(ctx)
	require.NoError(t, err)
	require.Equal(t, fixture.Node().ID, n.ID)

	slots := fixture.Node().Slots
	for i := 0; i < slots; i++ {
		ins := fixture.Instance()
		ins.ID = test.NewUUIDv7(t)
		ins.Chunk = c
		ins.FlavorVersion = c.Flavors[0].Versions[0]
		ins.Owner = c.Owner
		_, err := pg.DB.CreateInstance(ctx, ins, fixture.Node().ID)
		require.NoError(t, err)
	}

	_, err = pg.DB.BestNode(ctx)
	require.ErrorIs(t, err, apierrs.ErrNoSlotsAvailable)
}

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

	c.Flavors = []resource.Flavor{c.Flavors[0]}
	fmt.Println("gef", c.Flavors[0].ID)

	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	fmt.Println("af", c.Flavors[0].ID)

	expected := fixture.Instance()
	expected.Chunk = c
	expected.FlavorVersion = c.Flavors[0].Versions[0]
	expected.Flavor = c.Flavors[0]
	expected.Port = nil                             // port will not be saved when creating
	expected.FlavorVersion.FileHashes = nil         // will not be returned atm
	expected.Chunk.Owner = resource.User{}          // will not be returned atm
	expected.Chunk.Thumbnail = resource.Thumbnail{} // will not be returned atm
	expected.Flavor.Versions = nil                  // will not be returned atm
	expected.Owner = c.Owner
	expected.Owner.Email = "" // will not be returned atm

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
	)

	pg.Run(t, ctx)

	pg.InsertNode(t)
	pg.InsertMinecraftVersion(t)

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
			i.Owner.Email = "" // will not be returned atm
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
			i.Owner.Email = "" // will not be returned atm
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

func TestGetInstancesByNodeID(t *testing.T) {
	var (
		ctx    = context.Background()
		pg     = fixture.NewPostgres()
		nodeID = fixture.Node().ID
	)

	pg.Run(t, ctx)
	pg.InsertNode(t)
	pg.InsertMinecraftVersion(t)

	chunks := []resource.Chunk{
		fixture.Chunk(func(c *resource.Chunk) {
			c.ID = "01953e54-8ac5-7c1a-b468-dffdc26d2087"
			c.Name = "chunk1"
			c.Owner.Nickname = "user1"
			c.Owner.Email = "user1@example.com"
			c.Flavors = []resource.Flavor{
				fixture.Flavor(func(f *resource.Flavor) {
					f.ID = test.NewUUIDv7(t)
					f.Name = "flavor_" + test.RandHexStr(t)
				}),
			}
		}),
		fixture.Chunk(func(c *resource.Chunk) {
			c.ID = "01953e54-b686-764a-874f-dbc45b67152c"
			c.Name = "chunk2"
			c.Owner.Nickname = "user2"
			c.Owner.Email = "user2@example.com"
			c.Flavors = []resource.Flavor{
				fixture.Flavor(func(f *resource.Flavor) {
					f.ID = test.NewUUIDv7(t)
					f.Name = "flavor_" + test.RandHexStr(t)
				}),
			}
		}),
	}

	var expected []resource.Instance

	for i := range chunks {
		pg.CreateChunk(t, &chunks[i], fixture.CreateOptionsAll)

		v := chunks[i].Flavors[0].Versions[0]
		v.FileHashes = nil // not returned atm

		ins := resource.Instance{
			ID:            test.NewUUIDv7(t),
			Chunk:         chunks[i],
			Flavor:        chunks[i].Flavors[0],
			FlavorVersion: v,
			Address:       fixture.Node().Addr,
			State:         resource.InstanceStatePending,
			CreatedAt:     chunks[i].CreatedAt,
			UpdatedAt:     chunks[i].UpdatedAt,
			Owner:         chunks[i].Owner,
			OrderedBy:     "ordered_by",
		}

		ins.Chunk.Owner = resource.User{}          // will not be returned atm
		ins.Chunk.Thumbnail = resource.Thumbnail{} // will not be returned atm
		ins.Chunk.DeletedAt = nil                  // will not be returned atm
		ins.Owner.Email = ""                       // will not be returned atm
		ins.Flavor.Versions = nil                  // will not be returned atm

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

func TestCountInstancesByFlavorID(t *testing.T) {
	var (
		ctx    = context.Background()
		pg     = fixture.NewPostgres()
		nodeID = fixture.Node().ID
		c      = fixture.Chunk()
	)

	pg.Run(t, ctx)
	pg.InsertNode(t)
	pg.InsertMinecraftVersion(t)
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

	_, err := pg.DB.CreateInstance(ctx, ins1, nodeID)
	require.NoError(t, err)

	_, err = pg.DB.CreateInstance(ctx, ins2, nodeID)
	require.NoError(t, err)

	count, err := pg.DB.CountInstancesByFlavorVersionID(ctx, version.ID)
	require.NoError(t, err)

	require.Equal(t, uint(2), count)
}

// TODO: add test for applystatusreports
