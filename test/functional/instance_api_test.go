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
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/instance"
	"github.com/spacechunks/explorer/test"
	"github.com/spacechunks/explorer/test/functional/fixture"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestDiscoverInstances(t *testing.T) {
	nodeID := test.NewUUIDv7(t)

	tests := []struct {
		name        string
		nodeID      string
		input       []instance.Instance
		getExpected func([]instance.Instance) []*instancev1alpha1.Instance
		err         error
	}{
		{
			name:   "can discover instances",
			nodeID: nodeID,
			input: []instance.Instance{
				fixture.Instance(func(i *instance.Instance) {
					flavor := fixture.Flavor(func(f *chunk.Flavor) {
						f.ID = test.NewUUIDv7(t)
					})
					i.ID = "6566d325-8146-4532-b014-e13d7069af77"
					i.State = instance.StateRunning
					i.ChunkFlavor = flavor
					i.Chunk = fixture.Chunk(func(c *chunk.Chunk) {
						c.ID = test.NewUUIDv7(t)
						c.Flavors = []chunk.Flavor{flavor}
					})
				}),
				fixture.Instance(func(i *instance.Instance) {
					flavor := fixture.Flavor(func(f *chunk.Flavor) {
						f.ID = test.NewUUIDv7(t)
					})
					i.ID = "43fc4528-30ae-4003-9edf-8ab3bdae6c69"
					i.State = instance.StatePending
					i.ChunkFlavor = flavor
					i.Chunk = fixture.Chunk(func(c *chunk.Chunk) {
						c.ID = test.NewUUIDv7(t)
						c.Flavors = []chunk.Flavor{flavor}
					})
				}),
			},
			getExpected: func(instances []instance.Instance) []*instancev1alpha1.Instance {
				ret := make([]*instancev1alpha1.Instance, 0, len(instances))
				for _, ins := range instances {
					ret = append(ret, instance.FromDomain(ins))
				}
				return ret
			},
		},
		{
			name:   "wrong node id returns no instances",
			nodeID: "019556c6-ee21-7997-b97e-52e999e60a71",
			input: []instance.Instance{
				fixture.Instance(),
			},
			getExpected: func(instances []instance.Instance) []*instancev1alpha1.Instance {
				return nil
			},
		},
		{
			name:   "no node id returns error",
			nodeID: "",
			input:  []instance.Instance{},
			getExpected: func(instances []instance.Instance) []*instancev1alpha1.Instance {
				return nil
			},
			err: status.Error(codes.InvalidArgument, "node key is required"), // TODO: better error handling
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				pg  = fixture.NewPostgres()
			)

			fixture.RunControlPlane(t, pg)

			// FIXME: find better way to seed nodes
			_, err := pg.Pool.Exec(ctx, `INSERT INTO nodes (id, address) VALUES ($1, $2)`, nodeID, "198.51.100.1")
			require.NoError(t, err)

			for _, i := range tt.input {
				_, err = pg.DB.CreateChunk(ctx, i.Chunk)
				require.NoError(t, err)

				_, err = pg.DB.CreateFlavor(ctx, i.ChunkFlavor, i.Chunk.ID)
				require.NoError(t, err)

				_, err = pg.DB.CreateInstance(ctx, i, nodeID)
				require.NoError(t, err)
			}

			conn, err := grpc.NewClient(
				fixture.ControlPlaneAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			var (
				client   = instancev1alpha1.NewInstanceServiceClient(conn)
				expected = tt.getExpected(tt.input)
			)

			resp, err := client.DiscoverInstances(ctx, &instancev1alpha1.DiscoverInstanceRequest{
				NodeKey: &tt.nodeID,
			})

			if tt.err == nil {
				require.NoError(t, err)
				sort.Slice(expected, func(i, j int) bool {
					return strings.Compare(expected[i].GetId(), expected[j].GetId()) < 0
				})

				if d := cmp.Diff(expected, resp.Instances, protocmp.Transform()); d != "" {
					t.Fatalf("diff (-want +got):\n%s", d)
				}
				return
			}

			require.ErrorIs(t, err, tt.err)
		})
	}
}
