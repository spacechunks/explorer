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
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/instance"
	"github.com/spacechunks/explorer/controlplane/postgres"
	"github.com/spacechunks/explorer/internal/ptr"
	"github.com/spacechunks/explorer/test"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAPIListInstances(t *testing.T) {
	var (
		ctx    = context.Background()
		pg     = fixture.NewPostgres()
		nodeID = "0195c2f6-f40c-72df-a0f1-e468f1be77b1"
		c      = fixture.Chunk()
	)

	fixture.RunControlPlane(t, pg)

	// FIXME: find better way to seed nodes
	_, err := pg.Pool.Exec(ctx, `INSERT INTO nodes (id, address) VALUES ($1, $2)`, nodeID, "198.51.100.1")
	require.NoError(t, err)

	// make sure we only have one flavor, the fixture has 2 configured by default
	// but for this test we only we need one.
	c.Flavors = []chunk.Flavor{c.Flavors[0]}

	_, err = pg.DB.CreateChunk(ctx, fixture.Chunk())
	require.NoError(t, err)

	createdFlavor, err := pg.DB.CreateFlavor(ctx, fixture.Chunk().ID, fixture.Chunk().Flavors[0])
	require.NoError(t, err)

	ins := []instance.Instance{
		fixture.Instance(func(i *instance.Instance) {
			i.ID = test.NewUUIDv7(t)
			i.Chunk = fixture.Chunk()
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

	for _, i := range ins {
		_, err := pg.DB.CreateInstance(ctx, i, nodeID)
		require.NoError(t, err)
	}

	conn, err := grpc.NewClient(
		fixture.ControlPlaneAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	client := instancev1alpha1.NewInstanceServiceClient(conn)

	expected := make([]*instancev1alpha1.Instance, 0, len(ins))
	for _, i := range ins {
		expected = append(expected, instance.ToTransport(i))
	}

	resp, err := client.ListInstances(ctx, &instancev1alpha1.ListInstancesRequest{})
	require.NoError(t, err)

	if d := cmp.Diff(
		expected,
		resp.GetInstances(),
		protocmp.Transform(),
		test.IgnoredProtoFlavorFields,
	); d != "" {
		t.Fatalf("diff (-want +got):\n%s", d)
	}
}

func TestRunChunk(t *testing.T) {
	tests := []struct {
		name     string
		chunkID  string
		expected *instancev1alpha1.Instance
		err      error
	}{
		{
			name:    "can run chunk",
			chunkID: fixture.Chunk().ID,
			expected: &instancev1alpha1.Instance{
				Id: nil,
				Chunk: &chunkv1alpha1.Chunk{
					Id:          ptr.Pointer(fixture.Chunk().ID),
					Name:        ptr.Pointer(fixture.Chunk().Name),
					Description: ptr.Pointer(fixture.Chunk().Description),
					Tags:        fixture.Chunk().Tags,
					CreatedAt:   timestamppb.New(fixture.Chunk().CreatedAt),
					UpdatedAt:   timestamppb.New(fixture.Chunk().UpdatedAt),
				},
				Flavor: &chunkv1alpha1.Flavor{
					Name: ptr.Pointer(fixture.Chunk().Flavors[0].Name),
				},
				Ip:    ptr.Pointer("198.51.100.1"),
				State: ptr.Pointer(instancev1alpha1.InstanceState_PENDING),
			},
		},
		{
			name:    "chunk not found",
			chunkID: "93a3ee8a-4a6d-4f4f-b282-dcce199033c8",
			err:     postgres.ErrNotFound,
		},
		{
			name: "flavor not found",
			err:  errors.New("flavor not found"), // FIXME: better error handling
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx    = context.Background()
				pg     = fixture.NewPostgres()
				nodeID = "0195c2f6-f40c-72df-a0f1-e468f1be77b1"
			)

			fixture.RunControlPlane(t, pg)

			// FIXME: find better way to seed nodes
			_, err := pg.Pool.Exec(ctx, `INSERT INTO nodes (id, address) VALUES ($1, $2)`, nodeID, "198.51.100.1")
			require.NoError(t, err)

			_, err = pg.DB.CreateChunk(ctx, fixture.Chunk())
			require.NoError(t, err)

			createdFlavor, err := pg.DB.CreateFlavor(ctx, fixture.Chunk().ID, fixture.Chunk().Flavors[0])
			require.NoError(t, err)

			conn, err := grpc.NewClient(
				fixture.ControlPlaneAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			client := instancev1alpha1.NewInstanceServiceClient(conn)

			resp, err := client.RunChunk(ctx, &instancev1alpha1.RunChunkRequest{
				ChunkId:  &tt.chunkID,
				FlavorId: &createdFlavor.ID,
			})

			if tt.err == nil {
				require.NoError(t, err)
				tt.expected.Id = resp.GetInstance().Id
				if d := cmp.Diff(
					tt.expected,
					resp.GetInstance(),
					protocmp.Transform(),
					test.IgnoredProtoFlavorFields,
				); d != "" {
					t.Fatalf("diff (-want +got):\n%s", d)
				}
				return
			}

			require.ErrorAs(t, err, &tt.err)
		})
	}
}

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
					i.ChunkFlavor.Name = "f1"
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
					i.ChunkFlavor.Name = "f2"
					i.Chunk = fixture.Chunk(func(c *chunk.Chunk) {
						c.ID = test.NewUUIDv7(t)
						c.Flavors = []chunk.Flavor{flavor}
					})
				}),
			},
			getExpected: func(instances []instance.Instance) []*instancev1alpha1.Instance {
				ret := make([]*instancev1alpha1.Instance, 0, len(instances))
				for _, ins := range instances {
					ins.Port = nil // port will be nil at this point
					ret = append(ret, instance.ToTransport(ins))
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

				createdFlavor, err := pg.DB.CreateFlavor(ctx, i.Chunk.ID, i.ChunkFlavor)
				require.NoError(t, err)

				i.ChunkFlavor = createdFlavor

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

				if d := cmp.Diff(
					expected,
					resp.Instances,
					protocmp.Transform(),
					test.IgnoredProtoFlavorFields,
				); d != "" {
					t.Fatalf("diff (-want +got):\n%s", d)
				}
				return
			}

			require.ErrorIs(t, err, tt.err)
		})
	}
}

func TestReceiveInstanceStatusReports(t *testing.T) {
	tests := []struct {
		name     string
		input    instance.Instance
		report   instance.StatusReport
		expected instance.Instance
	}{
		{
			name:  "updates port and state successfully",
			input: fixture.Instance(),
			report: instance.StatusReport{
				InstanceID: fixture.Instance().ID,
				State:      instance.CreationFailed,
				Port:       420,
			},
			expected: fixture.Instance(func(i *instance.Instance) {
				i.State = instance.CreationFailed
				i.Port = ptr.Pointer(uint16(420))
			}),
		},
		{
			name:  "updates with state = DELETED removes instance",
			input: fixture.Instance(),
			report: instance.StatusReport{
				InstanceID: fixture.Instance().ID,
				State:      instance.StateDeleted,
			},
			expected: instance.Instance{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx    = context.Background()
				pg     = fixture.NewPostgres()
				nodeID = test.NewUUIDv7(t)
			)

			fixture.RunControlPlane(t, pg)

			ins := fixture.Instance()

			// FIXME: find better way to seed nodes
			_, err := pg.Pool.Exec(ctx, `INSERT INTO nodes (id, address) VALUES ($1, $2)`, nodeID, "198.51.100.1")
			require.NoError(t, err)

			_, err = pg.DB.CreateChunk(ctx, ins.Chunk)
			require.NoError(t, err)

			createdFlavor, err := pg.DB.CreateFlavor(ctx, ins.Chunk.ID, ins.ChunkFlavor)
			require.NoError(t, err)

			ins.ChunkFlavor = createdFlavor

			_, err = pg.DB.CreateInstance(ctx, ins, nodeID)
			require.NoError(t, err)

			conn, err := grpc.NewClient(
				fixture.ControlPlaneAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			client := instancev1alpha1.NewInstanceServiceClient(conn)

			_, err = client.ReceiveInstanceStatusReports(ctx, &instancev1alpha1.ReceiveInstanceStatusReportsRequest{
				Reports: []*instancev1alpha1.InstanceStatusReport{
					instance.StatusReportToTransport(tt.report),
				},
			})
			require.NoError(t, err)

			resp, err := client.DiscoverInstances(ctx, &instancev1alpha1.DiscoverInstanceRequest{
				NodeKey: &nodeID,
			})
			require.NoError(t, err)

			var expected []*instancev1alpha1.Instance
			if !reflect.DeepEqual(tt.expected, instance.Instance{}) {
				expected = []*instancev1alpha1.Instance{
					instance.ToTransport(tt.expected),
				}
			}

			if d := cmp.Diff(resp.Instances, expected, protocmp.Transform(), test.IgnoredProtoFlavorFields); d != "" {
				t.Fatalf("diff (-want +got):\n%s", d)
			}
		})
	}
}
