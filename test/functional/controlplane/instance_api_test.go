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

package controlplane

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	userv1alpha1 "github.com/spacechunks/explorer/api/user/v1alpha1"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/instance"
	"github.com/spacechunks/explorer/controlplane/postgres"
	"github.com/spacechunks/explorer/controlplane/resource"
	"github.com/spacechunks/explorer/internal/ptr"
	"github.com/spacechunks/explorer/test"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/stretchr/testify/require"
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
				cp  = fixture.NewControlPlane(t)
				ins = fixture.Instance()
			)

			cp.Run(t)
			cp.Postgres.InsertNode(t)

			if tt.create {
				cp.Postgres.CreateInstance(t, fixture.Node().ID, &ins)
				_, err := cp.Postgres.Pool.Exec(
					ctx,
					`UPDATE instances SET port = $1 WHERE id = $2`,
					ins.Port,
					ins.ID,
				)
				require.NoError(t, err)
			}

			cp.AddUserAPIKey(t, &ctx, ins.Owner)
			client := cp.InstanceClient(t)

			fmt.Println("AAAAA", ins.Owner.ID)

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
		cp  = fixture.NewControlPlane(t)
	)

	cp.Run(t)

	cp.Postgres.InsertNode(t)

	ins := []resource.Instance{
		fixture.Instance(func(i *resource.Instance) {
			i.ID = test.NewUUIDv7(t)
			i.Chunk = fixture.Chunk(func(c *resource.Chunk) {
				c.ID = test.NewUUIDv7(t)
				c.Flavors = []resource.Flavor{
					fixture.Flavor(func(f *resource.Flavor) {
						f.Name = "f1"
					}),
				}
			})
			i.FlavorVersion = i.Chunk.Flavors[0].Versions[0]
			i.Port = nil                     // port will not be saved when creating
			i.FlavorVersion.FileHashes = nil // not returned atm
		}),
		fixture.Instance(func(i *resource.Instance) {
			i.ID = test.NewUUIDv7(t)
			i.Chunk = fixture.Chunk(func(c *resource.Chunk) {
				c.ID = test.NewUUIDv7(t)
				c.Flavors = []resource.Flavor{
					fixture.Flavor(func(f *resource.Flavor) {
						f.Name = "f2"
					}),
				}
			})
			i.FlavorVersion = i.Chunk.Flavors[0].Versions[0]
			i.Port = nil                     // port will not be saved when creating
			i.FlavorVersion.FileHashes = nil // not returned atm
		}),
	}

	for idx := range ins {
		cp.Postgres.CreateInstance(t, fixture.Node().ID, &ins[idx])
	}

	cp.AddUserAPIKey(t, &ctx, ins[0].Owner)
	client := cp.InstanceClient(t)

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
		test.IgnoredProtoFlavorVersionFields,
		test.IgnoredProtoChunkFields,
		test.IgnoredProtoInstanceFields,
		test.IgnoredProtoUserFields,
	); d != "" {
		t.Fatalf("diff (-want +got):\n%s", d)
	}
}

func TestRunFlavorVersion(t *testing.T) {
	tests := []struct {
		name            string
		chunkID         string
		flavorVersionID string
		err             error
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
			name:            "flavor not found",
			flavorVersionID: "NOTFOUND",
			err:             apierrs.ErrFlavorVersionNotFound.GRPCStatus().Err(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				cp  = fixture.NewControlPlane(t)
				c   = fixture.Chunk()
			)

			cp.Run(t)

			cp.Postgres.InsertNode(t)
			cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)

			v := c.Flavors[0].Versions[0]

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
				FlavorVersion: &chunkv1alpha1.FlavorVersion{
					Id:               v.ID,
					Version:          v.Version,
					MinecraftVersion: fixture.MinecraftVersion,
					Hash:             v.Hash,
					FileHashes:       nil, // not returned atm
					BuildStatus:      chunkv1alpha1.BuildStatus(chunkv1alpha1.BuildStatus_value[string(v.BuildStatus)]),
					CreatedAt:        timestamppb.New(v.CreatedAt),
				},
				Owner: &userv1alpha1.User{
					Id:        c.Owner.ID,
					Nickname:  c.Owner.Nickname,
					CreatedAt: timestamppb.New(c.Owner.CreatedAt),
					UpdatedAt: timestamppb.New(c.Owner.UpdatedAt),
				},
				Ip:    fixture.Node().Addr.String(),
				State: instancev1alpha1.InstanceState_PENDING,
			}

			if tt.chunkID == "" {
				tt.chunkID = c.ID
			}

			if tt.flavorVersionID == "" {
				tt.flavorVersionID = c.Flavors[0].Versions[0].ID
			}

			cp.AddUserAPIKey(t, &ctx, c.Owner)
			client := cp.InstanceClient(t)

			resp, err := client.RunFlavorVersion(ctx, &instancev1alpha1.RunFlavorVersionRequest{
				ChunkId:         tt.chunkID,
				FlavorVersionId: tt.flavorVersionID,
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
				test.IgnoredProtoFlavorVersionFields,
				test.IgnoredProtoChunkFields,
				test.IgnoredProtoUserFields,
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
		input       []resource.Instance
		getExpected func([]resource.Instance) []*instancev1alpha1.Instance
		err         error
	}{
		{
			name:   "can discover instances",
			nodeID: fixture.Node().ID,
			input: []resource.Instance{
				fixture.Instance(func(i *resource.Instance) {
					i.ID = test.NewUUIDv7(t)
					i.Chunk = fixture.Chunk(func(c *resource.Chunk) {
						c.ID = test.NewUUIDv7(t)
						c.Flavors = []resource.Flavor{
							fixture.Flavor(func(f *resource.Flavor) {
								f.Name = "f1"
							}),
						}
					})
					i.FlavorVersion = i.Chunk.Flavors[0].Versions[0]
				}),
				fixture.Instance(func(i *resource.Instance) {
					i.ID = test.NewUUIDv7(t)
					i.Chunk = fixture.Chunk(func(c *resource.Chunk) {
						c.ID = test.NewUUIDv7(t)
						c.Flavors = []resource.Flavor{
							fixture.Flavor(func(f *resource.Flavor) {
								f.Name = "f2"
							}),
						}
					})
					i.FlavorVersion = i.Chunk.Flavors[0].Versions[0]
				}),
			},
			getExpected: func(instances []resource.Instance) []*instancev1alpha1.Instance {
				ret := make([]*instancev1alpha1.Instance, 0, len(instances))
				for _, ins := range instances {
					ins.Port = nil                     // port will be nil at this point
					ins.FlavorVersion.FileHashes = nil // not returned atm
					ret = append(ret, instance.ToTransport(ins))
				}
				return ret
			},
		},
		{
			name:   "wrong node id returns no instances",
			nodeID: "019556c6-ee21-7997-b97e-52e999e60a71",
			input: []resource.Instance{
				fixture.Instance(),
			},
			getExpected: func(instances []resource.Instance) []*instancev1alpha1.Instance {
				return nil
			},
		},
		{
			name:   "no node id returns error",
			nodeID: "",
			input: []resource.Instance{
				fixture.Instance(),
			},
			getExpected: func(instances []resource.Instance) []*instancev1alpha1.Instance {
				return nil
			},
			err: apierrs.ErrNodeKeyMissing.GRPCStatus().Err(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				cp  = fixture.NewControlPlane(t)
			)

			cp.Run(t)
			cp.Postgres.InsertNode(t)

			for idx := range tt.input {
				cp.Postgres.CreateInstance(t, fixture.Node().ID, &tt.input[idx])
			}

			cp.AddUserAPIKey(t, &ctx, tt.input[0].Owner)

			var (
				client   = cp.InstanceClient(t)
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
					test.IgnoredProtoFlavorVersionFields,
					test.IgnoredProtoChunkFields,
					test.IgnoredProtoUserFields,
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
		report   resource.InstanceStatusReport
		expected resource.Instance
	}{
		{
			name: "updates port and state successfully",
			report: resource.InstanceStatusReport{
				InstanceID: fixture.Instance().ID,
				State:      resource.InstanceCreationFailed,
				Port:       420,
			},
			expected: fixture.Instance(func(i *resource.Instance) {
				i.State = resource.InstanceCreationFailed
				i.Port = ptr.Pointer(uint16(420))
				i.FlavorVersion.FileHashes = nil // not returned atm
			}),
		},
		{
			name: "updates with state = DELETED removes instance",
			report: resource.InstanceStatusReport{
				InstanceID: fixture.Instance().ID,
				State:      resource.InstanceStateDeleted,
			},
			expected: resource.Instance{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				cp  = fixture.NewControlPlane(t)
				ins = fixture.Instance()
			)

			cp.Run(t)

			cp.Postgres.InsertNode(t)
			cp.Postgres.CreateInstance(t, fixture.Node().ID, &ins)

			cp.AddUserAPIKey(t, &ctx, ins.Owner)
			client := cp.InstanceClient(t)

			tt.report.InstanceID = ins.ID

			_, err := client.ReceiveInstanceStatusReports(ctx, &instancev1alpha1.ReceiveInstanceStatusReportsRequest{
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
			if !reflect.DeepEqual(tt.expected, resource.Instance{}) {
				tt.expected.Owner = ins.Owner
				expected = []*instancev1alpha1.Instance{
					instance.ToTransport(tt.expected),
				}
			}

			if d := cmp.Diff(
				resp.Instances,
				expected,
				protocmp.Transform(),
				test.IgnoredProtoFlavorVersionFields,
				test.IgnoredProtoChunkFields,
				test.IgnoredProtoUserFields,
			); d != "" {
				t.Fatalf("diff (-want +got):\n%s", d)
			}
		})
	}
}
