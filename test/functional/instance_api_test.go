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
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	"github.com/spacechunks/explorer/controlplane/chunk"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/instance"
	"github.com/spacechunks/explorer/controlplane/postgres"
	"github.com/spacechunks/explorer/internal/ptr"
	"github.com/spacechunks/explorer/test"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestGetInstance(t *testing.T) {
	tests := []struct {
		name   string
		create bool
		err    error
	}{
		{
			name:   "works",
			create: true,
		},
		{
			name: "not found",
			err:  apierrs.ErrInstanceNotFound.GRPCStatus().Err(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				pg  = fixture.NewPostgres()
				ins = fixture.Instance()
			)

			fixture.RunControlPlane(t, pg)

			pg.InsertNode(t)

			if tt.create {
				ins = fixture.Instance()
				pg.CreateInstance(t, fixture.Node().ID, &ins)
				_, err := pg.Pool.Exec(
					ctx,
					`UPDATE instances SET port = $1 WHERE id = $2`,
					ins.Port,
					ins.ID,
				)
				require.NoError(t, err)
			}

			conn, err := grpc.NewClient(
				fixture.ControlPlaneAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			client := instancev1alpha1.NewInstanceServiceClient(conn)

			resp, err := client.GetInstance(ctx, &instancev1alpha1.GetInstanceRequest{
				Id: ins.ID,
			})

			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}

			require.NoError(t, err)

			if d := cmp.Diff(
				instance.ToTransport(ins),
				resp.GetInstance(),
				protocmp.Transform(),
				test.IgnoredProtoFlavorFields,
				test.IgnoredProtoChunkFields,
			); d != "" {
				t.Fatalf("diff (-want +got):\n%s", d)
			}
		})
	}
}

func TestAPIListInstances(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
	)

	fixture.RunControlPlane(t, pg)

	pg.InsertNode(t)

	ins := []instance.Instance{
		fixture.Instance(func(i *instance.Instance) {
			i.ID = test.NewUUIDv7(t)
			i.Chunk = fixture.Chunk(func(c *chunk.Chunk) {
				c.ID = test.NewUUIDv7(t)
				c.Flavors = []chunk.Flavor{
					fixture.Flavor(func(f *chunk.Flavor) {
						f.Name = "f1"
					}),
				}
			})
			i.ChunkFlavor = i.Chunk.Flavors[0]
			i.Port = nil // port will not be saved when creating
		}),
		fixture.Instance(func(i *instance.Instance) {
			i.ID = test.NewUUIDv7(t)
			i.Chunk = fixture.Chunk(func(c *chunk.Chunk) {
				c.ID = test.NewUUIDv7(t)
				c.Flavors = []chunk.Flavor{
					fixture.Flavor(func(f *chunk.Flavor) {
						f.Name = "f2"
					}),
				}
			})
			i.ChunkFlavor = i.Chunk.Flavors[0]
			i.Port = nil // port will not be saved when creating
		}),
	}

	for _, i := range ins {
		pg.CreateInstance(t, fixture.Node().ID, &i)
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

	sort.Slice(expected, func(i, j int) bool {
		return strings.Compare(expected[i].GetId(), expected[j].GetId()) < 0
	})

	sort.Slice(resp.GetInstances(), func(i, j int) bool {
		return strings.Compare(resp.GetInstances()[i].GetId(), resp.GetInstances()[j].GetId()) < 0
	})

	if d := cmp.Diff(
		expected,
		resp.GetInstances(),
		protocmp.Transform(),
		test.IgnoredProtoFlavorFields,
		test.IgnoredProtoChunkFields,
		test.IgnoredProtoInstanceFields,
	); d != "" {
		t.Fatalf("diff (-want +got):\n%s", d)
	}
}

func TestRunChunk(t *testing.T) {
	tests := []struct {
		name     string
		chunkID  string
		flavorID string
		err      error
	}{
		{
			name: "can run chunk",
		},
		{
			name:    "chunk not found",
			chunkID: "93a3ee8a-4a6d-4f4f-b282-dcce199033c8",
			err:     postgres.ErrNotFound,
		},
		{
			name:     "flavor not found",
			flavorID: "NOTFOUND",
			err:      apierrs.ErrFlavorNotFound.GRPCStatus().Err(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				pg  = fixture.NewPostgres()
				c   = fixture.Chunk()
			)

			fixture.RunControlPlane(t, pg)

			pg.InsertNode(t)
			pg.CreateChunk(t, &c)

			expected := &instancev1alpha1.Instance{
				Id: "",
				Chunk: &chunkv1alpha1.Chunk{
					Id:          c.ID,
					Name:        c.Name,
					Description: c.Description,
					Tags:        c.Tags,
					CreatedAt:   timestamppb.New(c.CreatedAt),
					UpdatedAt:   timestamppb.New(c.UpdatedAt),
				},
				Flavor: &chunkv1alpha1.Flavor{
					Name: c.Flavors[0].Name,
				},
				Ip:    fixture.Node().Addr,
				State: instancev1alpha1.InstanceState_PENDING,
			}

			if tt.chunkID == "" {
				tt.chunkID = c.ID
			}

			if tt.flavorID == "" {
				tt.flavorID = c.Flavors[0].ID
			}

			conn, err := grpc.NewClient(
				fixture.ControlPlaneAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			client := instancev1alpha1.NewInstanceServiceClient(conn)

			resp, err := client.RunChunk(ctx, &instancev1alpha1.RunChunkRequest{
				ChunkId:  tt.chunkID,
				FlavorId: tt.flavorID,
			})

			if tt.err != nil {
				require.ErrorAs(t, err, &tt.err)
				return
			}

			require.NoError(t, err)

			if d := cmp.Diff(
				expected,
				resp.GetInstance(),
				protocmp.Transform(),
				test.IgnoredProtoInstanceFields,
				test.IgnoredProtoFlavorFields,
				test.IgnoredProtoChunkFields,
			); d != "" {
				t.Fatalf("diff (-want +got):\n%s", d)
			}
		})
	}
}

func TestDiscoverInstances(t *testing.T) {
	tests := []struct {
		name        string
		nodeID      string
		input       []instance.Instance
		getExpected func([]instance.Instance) []*instancev1alpha1.Instance
		err         error
	}{
		{
			name:   "can discover instances",
			nodeID: fixture.Node().ID,
			input: []instance.Instance{
				fixture.Instance(func(i *instance.Instance) {
					i.ID = test.NewUUIDv7(t)
					i.Chunk = fixture.Chunk(func(c *chunk.Chunk) {
						c.ID = test.NewUUIDv7(t)
						c.Flavors = []chunk.Flavor{
							fixture.Flavor(func(f *chunk.Flavor) {
								f.Name = "f1"
							}),
						}
					})
					i.ChunkFlavor = i.Chunk.Flavors[0]
				}),
				fixture.Instance(func(i *instance.Instance) {
					i.ID = test.NewUUIDv7(t)
					i.Chunk = fixture.Chunk(func(c *chunk.Chunk) {
						c.ID = test.NewUUIDv7(t)
						c.Flavors = []chunk.Flavor{
							fixture.Flavor(func(f *chunk.Flavor) {
								f.Name = "f2"
							}),
						}
					})
					i.ChunkFlavor = i.Chunk.Flavors[0]
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
			err: apierrs.ErrNodeKeyMissing.GRPCStatus().Err(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				pg  = fixture.NewPostgres()
			)

			fixture.RunControlPlane(t, pg)

			pg.InsertNode(t)

			for _, i := range tt.input {
				pg.CreateInstance(t, fixture.Node().ID, &i)
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
				NodeKey: tt.nodeID,
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
					test.IgnoredProtoInstanceFields,
					test.IgnoredProtoFlavorFields,
					test.IgnoredProtoChunkFields,
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
				ctx = context.Background()
				pg  = fixture.NewPostgres()
				ins = fixture.Instance()
			)

			fixture.RunControlPlane(t, pg)

			pg.InsertNode(t)
			pg.CreateInstance(t, fixture.Node().ID, &ins)

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
				NodeKey: fixture.Node().ID,
			})
			require.NoError(t, err)

			var expected []*instancev1alpha1.Instance
			if !reflect.DeepEqual(tt.expected, instance.Instance{}) {
				expected = []*instancev1alpha1.Instance{
					instance.ToTransport(tt.expected),
				}
			}

			if d := cmp.Diff(
				resp.Instances,
				expected,
				protocmp.Transform(),
				test.IgnoredProtoFlavorFields,
				test.IgnoredProtoChunkFields,
			); d != "" {
				t.Fatalf("diff (-want +got):\n%s", d)
			}
		})
	}
}
