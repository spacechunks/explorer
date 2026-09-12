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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/jackc/pgx/v5"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	"github.com/spacechunks/explorer/controlplane/blob"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/internal/file"
	"github.com/spacechunks/explorer/internal/image"
	imgtestdata "github.com/spacechunks/explorer/internal/image/testdata"
	"github.com/spacechunks/explorer/internal/resource"
	"github.com/spacechunks/explorer/internal/resource/codec"
	"github.com/spacechunks/explorer/internal/tarhelper"
	"github.com/spacechunks/explorer/test"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/spacechunks/explorer/test/functional/controlplane/testdata"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/xxh3"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestAPICreateChunk(t *testing.T) {
	tests := []struct {
		name           string
		expected       resource.Chunk
		err            error
		errCode        codes.Code
		errMsgContains string
	}{
		{
			name: "works",
			expected: fixture.Chunk(func(c *resource.Chunk) {
				c.Flavors = nil
				c.Thumbnail = resource.Thumbnail{}
			}),
		},
		{
			name: "name too long",
			expected: fixture.Chunk(func(c *resource.Chunk) {
				c.Name = strings.Repeat("a", resource.MaxChunkNameChars+1)
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "name",
		},
		{
			name: "description too long",
			expected: fixture.Chunk(func(c *resource.Chunk) {
				c.Description = strings.Repeat("a", resource.MaxChunkDescriptionChars+1)
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "description",
		},
		{
			name: "too many tags",
			expected: fixture.Chunk(func(c *resource.Chunk) {
				c.Tags = slices.Repeat([]string{"a"}, resource.MaxChunkTags+1)
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "tags",
		},
		{
			name: "empty name does not work",
			expected: fixture.Chunk(func(c *resource.Chunk) {
				c.Name = ""
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "name",
		},
		{
			name: "name starting with space does not work",
			expected: fixture.Chunk(func(c *resource.Chunk) {
				c.Name = " hello"
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name: "name starting with .. does not work",
			expected: fixture.Chunk(func(c *resource.Chunk) {
				c.Name = "..hello"
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name: "name starting with ../ does not work",
			expected: fixture.Chunk(func(c *resource.Chunk) {
				c.Name = "../hello"
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name: "name starting with ... does not work",
			expected: fixture.Chunk(func(c *resource.Chunk) {
				c.Name = "...hello"
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name: "name ending with space oes not work",
			expected: fixture.Chunk(func(c *resource.Chunk) {
				c.Name = "hello "
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name: "name ending with .. does not work",
			expected: fixture.Chunk(func(c *resource.Chunk) {
				c.Name = "hello.."
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name: "name ending with /.. does not work",
			expected: fixture.Chunk(func(c *resource.Chunk) {
				c.Name = "hello/.."
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name: "name containing /../ does not work",
			expected: fixture.Chunk(func(c *resource.Chunk) {
				c.Name = "hello/../world"
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name: "name containing / does not work",
			expected: fixture.Chunk(func(c *resource.Chunk) {
				c.Name = "hello/world"
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name: "tag starts with -",
			expected: fixture.Chunk(func(c *resource.Chunk) {
				c.Tags = []string{"valid", "-invalid"}
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "tags: tags can only contain lower-case ascii letters",
		},
		{
			name: "tag ends with -",
			expected: fixture.Chunk(func(c *resource.Chunk) {
				c.Tags = []string{"valid", "invalid-"}
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "tags: tags can only contain lower-case ascii letters",
		},
		{
			name: "tag contains space",
			expected: fixture.Chunk(func(c *resource.Chunk) {
				c.Tags = []string{"valid", "inv alid"}
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "tags: tags can only contain lower-case ascii letters",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				cp  = fixture.NewControlPlane(t)
				u   = fixture.User()
			)

			cp.Run(t)
			client := cp.ChunkClient(t)

			cp.Postgres.CreateUser(t, &u)
			cp.AddUserAPIKey(t, &ctx, u)

			resp, err := client.CreateChunk(ctx, &chunkv1alpha1.CreateChunkRequest{
				Name:        tt.expected.Name,
				Description: tt.expected.Description,
				Tags:        tt.expected.Tags,
			})

			if tt.errCode != 0 {
				st, ok := status.FromError(err)
				require.True(t, ok)

				require.Equal(t, tt.errCode, st.Code())
				require.Contains(t, st.Message(), tt.errMsgContains)
				return
			}

			require.NoError(t, err)

			tt.expected.Owner = u

			if d := cmp.Diff(
				codec.ChunkToTransport(tt.expected),
				resp.GetChunk(),
				protocmp.Transform(),
				test.IgnoredProtoChunkFields,
				test.IgnoredProtoUserFields,
			); d != "" {
				t.Fatalf("diff (-want +got):\n%s", d)
			}
		})
	}
}

func TestGetChunk(t *testing.T) {
	tests := []struct {
		name           string
		chunkID        string
		errCode        codes.Code
		errMsgContains string
		err            error
	}{
		{
			name: "works",
		},
		{
			name:           "not found",
			chunkID:        test.NewUUIDv7(t),
			errCode:        codes.NotFound,
			errMsgContains: "chunk does not exist",
		},
		{
			name:           "invalid id",
			chunkID:        "invalid",
			errCode:        codes.InvalidArgument,
			errMsgContains: "id: must be a valid UUID",
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

			if tt.chunkID == "" {
				cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)
				tt.chunkID = c.ID
			} else {
				cp.Postgres.CreateUser(t, &c.Owner)
			}

			cp.AddUserAPIKey(t, &ctx, c.Owner)
			client := cp.ChunkClient(t)

			resp, err := client.GetChunk(ctx, &chunkv1alpha1.GetChunkRequest{
				Id: tt.chunkID,
			})

			if tt.errCode != 0 {
				st, ok := status.FromError(err)
				require.True(t, ok)

				require.Equal(t, tt.errCode, st.Code())
				require.Contains(t, st.Message(), tt.errMsgContains)
				return
			}

			require.NoError(t, err)

			if d := cmp.Diff(
				codec.ChunkToTransport(c),
				resp.GetChunk(),
				protocmp.Transform(),
				test.IgnoredProtoChunkFields,
				test.IgnoredProtoFlavorFields,
				test.IgnoredProtoFlavorVersionFields,
				test.IgnoredProtoUserFields,
			); d != "" {
				t.Fatalf("diff (-want +got):\n%s", d)
			}
		})
	}
}

func TestListChunks(t *testing.T) {
	var (
		ctx = context.Background()
		cp  = fixture.NewControlPlane(t)
		u   = fixture.User()
	)

	cp.Run(t)
	client := cp.ChunkClient(t)

	chunks := make([]resource.Chunk, 0, 13)
	for idx := range 12 {
		chunks = append(chunks, fixture.Chunk(func(c *resource.Chunk) {
			c.ID = test.NewUUIDv7(t)
			c.Owner = u
			c.Flavors = []resource.Flavor{
				fixture.Flavor(func(f *resource.Flavor) {
					f.ID = test.NewUUIDv7(t)
					f.Name = fmt.Sprintf("flavor-%d", idx)
				}),
			}
		}))
	}
	chunks = append(chunks, fixture.Chunk(func(c *resource.Chunk) { // this one should not appear
		c.ID = test.NewUUIDv7(t)
		c.Owner = u
		c.DeletedAt = new(time.Now())
	}))

	for i := range chunks {
		cp.Postgres.CreateChunk(t, &chunks[i], fixture.CreateOptionsAll)
	}

	cp.AddUserAPIKey(t, &ctx, chunks[0].Owner)

	resp, err := client.ListChunks(ctx, &chunkv1alpha1.ListChunksRequest{})
	require.NoError(t, err)
	require.Len(t, resp.GetChunks(), 10)
	require.NotEmpty(t, resp.GetNextPageToken())
	require.Equal(t, resp.GetChunks()[9].GetId(), resp.GetNextPageToken())

	expected := make([]*chunkv1alpha1.Chunk, 0)
	for _, c := range chunks {
		if c.DeletedAt != nil {
			continue
		}
		expected = append(expected, codec.ChunkToTransport(c))
	}

	sort.Slice(expected, func(i, j int) bool {
		return strings.Compare(expected[i].GetId(), expected[j].GetId()) < 0
	})

	sort.Slice(resp.GetChunks(), func(i, j int) bool {
		return strings.Compare(resp.GetChunks()[i].GetId(), resp.GetChunks()[j].GetId()) < 0
	})

	if d := cmp.Diff(
		expected[:10],
		resp.GetChunks(),
		protocmp.Transform(),
		test.IgnoredProtoChunkFields,
		test.IgnoredProtoFlavorFields,
		test.IgnoredProtoFlavorVersionFields,
		test.IgnoredProtoUserFields,
	); d != "" {
		t.Fatalf("diff (-want +got):\n%s", d)
	}

	secondPage, err := client.ListChunks(ctx, &chunkv1alpha1.ListChunksRequest{
		PageToken: resp.GetNextPageToken(),
	})
	require.NoError(t, err)
	require.Len(t, secondPage.GetChunks(), 2)
	require.Empty(t, secondPage.GetNextPageToken())

	_, err = client.ListChunks(ctx, &chunkv1alpha1.ListChunksRequest{
		PageToken: "invalid",
	})
	require.ErrorIs(t, err, apierrs.ErrInvalidPageToken.GRPCStatus().Err())

	sort.Slice(secondPage.GetChunks(), func(i, j int) bool {
		return strings.Compare(secondPage.GetChunks()[i].GetId(), secondPage.GetChunks()[j].GetId()) < 0
	})

	if d := cmp.Diff(
		expected[10:],
		secondPage.GetChunks(),
		protocmp.Transform(),
		test.IgnoredProtoChunkFields,
		test.IgnoredProtoFlavorFields,
		test.IgnoredProtoFlavorVersionFields,
		test.IgnoredProtoUserFields,
	); d != "" {
		t.Fatalf("second page diff (-want +got):\n%s", d)
	}
}

func TestUpdateChunk(t *testing.T) {
	tests := []struct {
		name           string
		c              *resource.Chunk
		req            *chunkv1alpha1.UpdateChunkRequest
		errCode        codes.Code
		errMsgContains string
	}{
		{
			name: "update all fields",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Name:        "new-name",
				Description: "new-description",
				Tags:        []string{"new-tags"},
			},
		},
		{
			name: "update name",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Name: "new-name",
			},
		},
		{
			name: "update description",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Description: "new-description",
			},
		},
		{
			name: "update tags",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Tags: []string{"new-tags"},
			},
		},
		{
			name: "not found",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Id:          test.NewUUIDv7(t),
				Name:        "new-name",
				Description: "new-description",
				Tags:        []string{"new-tags"},
			},
			errCode:        codes.NotFound,
			errMsgContains: "chunk does not exist",
		},
		{
			name: "name too long",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Name: strings.Repeat("a", resource.MaxChunkNameChars+1),
			},
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: must be at most 50 characters",
		},
		{
			name: "description too long",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Description: strings.Repeat("a", resource.MaxChunkDescriptionChars+1),
			},
			errCode:        codes.InvalidArgument,
			errMsgContains: "description: must be at most 100 characters",
		},
		{
			name: "too many tags",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Tags: slices.Repeat([]string{"a"}, resource.MaxChunkTags+1),
			},
			errCode:        codes.InvalidArgument,
			errMsgContains: "tags: must contain no more than 4 item(s)",
		},
		{
			name: "invalid chunk id",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Id: "invalid",
			},
			errCode:        codes.InvalidArgument,
			errMsgContains: "id: must be a valid UUID",
		},
		{
			name: "chunk not found because it's deleted",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Id:   fixture.Chunk().ID,
				Name: "new-name",
			},
			c: new(fixture.Chunk(func(tmp *resource.Chunk) {
				tmp.DeletedAt = new(time.Time)
			})),
			errCode:        codes.NotFound,
			errMsgContains: "chunk does not exist",
		},
		{
			name: "name starting with space does not work",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Id:   fixture.Chunk().ID,
				Name: " hello",
			},
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name: "name starting with .. does not work",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Id:   fixture.Chunk().ID,
				Name: "..hello",
			},
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name: "name starting with ../ does not work",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Id:   fixture.Chunk().ID,
				Name: "../hello",
			},
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name: "name starting with ... does not work",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Id:   fixture.Chunk().ID,
				Name: "...hello",
			},
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name: "name ending with space oes not work",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Id:   fixture.Chunk().ID,
				Name: "hello ",
			},
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name: "name ending with .. does not work",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Id:   fixture.Chunk().ID,
				Name: "hello..",
			},
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name: "name ending with /.. does not work",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Id:   fixture.Chunk().ID,
				Name: "hello/..",
			},
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name: "name containing /../ does not work",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Id:   fixture.Chunk().ID,
				Name: "hello/../world",
			},
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name: "name containing / does not work",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Id:   fixture.Chunk().ID,
				Name: "hello/world",
			},
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name: "tag starts with -",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Id:   fixture.Chunk().ID,
				Tags: []string{"valid", "-invalid"},
			},
			errCode:        codes.InvalidArgument,
			errMsgContains: "tags: tags can only contain lower-case ascii letters",
		},
		{
			name: "tag ends with -",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Id:   fixture.Chunk().ID,
				Tags: []string{"valid", "invalid-"},
			},
			errCode:        codes.InvalidArgument,
			errMsgContains: "tags: tags can only contain lower-case ascii letters",
		},
		{
			name: "tag contains space",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Id:   fixture.Chunk().ID,
				Tags: []string{"valid", "inv alid"},
			},
			errCode:        codes.InvalidArgument,
			errMsgContains: "tags: tags can only contain lower-case ascii letters",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				cp  = fixture.NewControlPlane(t)
			)

			cp.Run(t)

			if tt.c == nil {
				tt.c = new(fixture.Chunk())
			}

			cp.Postgres.CreateChunk(t, tt.c, fixture.CreateOptionsAll)

			if tt.req.Id == "" {
				tt.req.Id = tt.c.ID
			}

			client := cp.ChunkClient(t)
			cp.AddUserAPIKey(t, &ctx, tt.c.Owner)

			resp, err := client.UpdateChunk(ctx, tt.req)

			if tt.errCode != 0 {
				st, ok := status.FromError(err)
				require.True(t, ok)

				require.Equal(t, tt.errCode, st.Code())
				require.Contains(t, st.Message(), tt.errMsgContains)
				return
			}

			require.NoError(t, err)

			expected := codec.ChunkToTransport(*tt.c)

			if tt.req.Name != "" {
				expected.Name = tt.req.Name
			}

			if tt.req.Description != "" {
				expected.Description = tt.req.Description
			}

			if tt.req.Tags != nil {
				expected.Tags = tt.req.Tags
			}

			if d := cmp.Diff(
				expected,
				resp.GetChunk(),
				protocmp.Transform(),
				test.IgnoredProtoChunkFields,
				test.IgnoredProtoFlavorFields,
				test.IgnoredProtoFlavorVersionFields,
				test.IgnoredProtoUserFields,
			); d != "" {
				t.Fatalf("diff (-want +got):\n%s", d)
			}
		})
	}
}

func TestCreateFlavor(t *testing.T) {
	c := fixture.Chunk()
	tests := []struct {
		name           string
		flavorName     string
		chunkID        string
		errCode        codes.Code
		errMsgContains string
	}{
		{
			name:       "works",
			flavorName: fixture.Flavor().Name,
		},
		{
			name:           "invalid chunk id",
			flavorName:     fixture.Flavor().Name,
			chunkID:        "invalid",
			errCode:        codes.InvalidArgument,
			errMsgContains: "id: must be a valid UUID",
		},
		{
			name:           "invalid flavor name",
			flavorName:     strings.Repeat("a", 26),
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: must be at most 25 characters",
		},
		{
			name:           "name starting with space does not work",
			flavorName:     " hello",
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name:           "name starting with .. does not work",
			flavorName:     "..hello",
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name:           "name starting with ../ does not work",
			flavorName:     "../hello",
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name:           "name starting with ... does not work",
			flavorName:     "...hello",
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name:           "name ending with space oes not work",
			flavorName:     "hello ",
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name:           "name ending with .. does not work",
			flavorName:     "hello..",
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name:           "name ending with /.. does not work",
			flavorName:     "hello/..",
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name:           "name containing /../ does not work",
			flavorName:     "hello/../world",
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
		{
			name:           "name containing / does not work",
			flavorName:     "hello/world",
			errCode:        codes.InvalidArgument,
			errMsgContains: "name: names cannot start or end with a space",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				cp  = fixture.NewControlPlane(t)
			)

			cp.Run(t)

			cp.Postgres.CreateChunk(t, &c, fixture.CreateOptions{
				WithOwner: true,
			})

			if tt.chunkID == "" {
				tt.chunkID = c.ID
			}

			cp.AddUserAPIKey(t, &ctx, c.Owner)
			client := cp.ChunkClient(t)

			resp, err := client.CreateFlavor(ctx, &chunkv1alpha1.CreateFlavorRequest{
				ChunkId: tt.chunkID,
				Name:    tt.flavorName,
			})

			if tt.errCode != 0 {
				st, ok := status.FromError(err)
				require.True(t, ok)

				require.Equal(t, tt.errCode, st.Code())
				require.Contains(t, st.Message(), tt.errMsgContains)
				return
			}

			require.NoError(t, err)
			expected := &chunkv1alpha1.Flavor{
				Id:   c.ID,
				Name: tt.flavorName,
			}

			if d := cmp.Diff(
				resp.GetFlavor(),
				expected,
				protocmp.Transform(),
				test.IgnoredProtoFlavorFields,
			); d != "" {
				t.Fatalf("CreateFlavorResponse mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestCreateFlavorVersion(t *testing.T) {
	c := fixture.Chunk()

	cleanedPathVersion := func() resource.FlavorVersion {
		return fixture.FlavorVersion(func(v *resource.FlavorVersion) {
			v.Version = "v2"
			v.FileHashes = []file.Hash{
				{
					Path: "paper.yml",
					Hash: "pppppppppppppppp",
				},
				{
					Path: "server.properties",
					Hash: "cccccccccccccccc",
				},
				{
					Path: "plugins/myplugin.jar",
					Hash: "yyyyyyyyyyyyyyyy",
				},
			}
		})
	}

	uncleanPathVersion := func() resource.FlavorVersion {
		version := cleanedPathVersion()
		version.FileHashes = []file.Hash{
			{
				Path: "paper.yml",
				Hash: "pppppppppppppppp",
			},
			{
				Path: "plugins/myplugin.jar",
				Hash: "yyyyyyyyyyyyyyyy",
			},
			{
				Path: "plugins/../server.properties",
				Hash: "cccccccccccccccc",
			},
		}
		return version
	}

	tests := []struct {
		name            string
		prevVersion     *resource.FlavorVersion
		newVersion      resource.FlavorVersion
		expectedVersion *resource.FlavorVersion
		err             error
		errCode         codes.Code
		errMsgContains  string
		badRequest      *errdetails.BadRequest
	}{
		{
			name:       "create initial version",
			newVersion: fixture.FlavorVersion(),
		},
		{
			name:        "create second version with changed files",
			prevVersion: new(fixture.FlavorVersion()),
			newVersion: fixture.FlavorVersion(func(v *resource.FlavorVersion) {
				v.Version = "v2"
				v.FileHashes = []file.Hash{
					{
						Path: "paper.yml",
						Hash: "pppppppppppppppp",
					},
					{
						Path: "server.properties",
						Hash: "cccccccccccccccc",
					},
					{
						Path: "plugins/myplugin.jar",
						Hash: "yyyyyyyyyyyyyyyy",
					},
				}
			}),
		},
		{
			name:            "cleans paths",
			prevVersion:     new(fixture.FlavorVersion()),
			newVersion:      uncleanPathVersion(),
			expectedVersion: new(cleanedPathVersion()),
		},
		{
			name:        "invalid paths",
			prevVersion: new(fixture.FlavorVersion()),
			newVersion: fixture.FlavorVersion(func(v *resource.FlavorVersion) {
				v.Version = "v2"
				v.FileHashes = []file.Hash{
					{
						Path: "../server.properties",
						Hash: "cccccccccccccccc",
					},
					{
						Path: "/plugins/myplugin.jar",
						Hash: "yyyyyyyyyyyyyyyy",
					},
				}
			}),
			err: apierrs.InvalidPath(apierrs.InvalidPathViolation{
				Field: "version.file_hashes[0].path",
				Path:  "../server.properties",
			}).GRPCStatus().Err(),
			badRequest: &errdetails.BadRequest{
				FieldViolations: []*errdetails.BadRequest_FieldViolation{
					{
						Field:       "version.file_hashes[0].path",
						Description: `path "../server.properties" must not be absolute or escape the flavor version root`,
						Reason:      "INVALID_PATH",
					},
					{
						Field:       "version.file_hashes[1].path",
						Description: `path "/plugins/myplugin.jar" must not be absolute or escape the flavor version root`,
						Reason:      "INVALID_PATH",
					},
				},
			},
		},
		{
			name:        "version already exists",
			prevVersion: new(fixture.FlavorVersion()),
			newVersion:  fixture.FlavorVersion(),
			err:         apierrs.ErrFlavorVersionExists.GRPCStatus().Err(),
		},
		{
			name:        "version hash mismatch",
			prevVersion: new(fixture.FlavorVersion()),
			newVersion: fixture.FlavorVersion(func(v *resource.FlavorVersion) {
				v.Version = "v2"
				v.Hash = "wrong-hash"
			}),
			err: apierrs.ErrHashMismatch.GRPCStatus().Err(),
		},
		{
			name:        "unsupported minecraft version",
			prevVersion: new(fixture.FlavorVersion()),
			newVersion: fixture.FlavorVersion(func(v *resource.FlavorVersion) {
				v.Version = "v2"
				v.MinecraftVersion = "abcdef"
			}),
			err: apierrs.ErrMinecraftVersionNotSupported.GRPCStatus().Err(),
		},
		{
			name:        "version starting with space does not work",
			prevVersion: new(fixture.FlavorVersion()),
			newVersion: fixture.FlavorVersion(func(v *resource.FlavorVersion) {
				v.Version = " hello"
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "version: version cannot start or end with a space",
		},
		{
			name:        "version starting with .. does not work",
			prevVersion: new(fixture.FlavorVersion()),
			newVersion: fixture.FlavorVersion(func(v *resource.FlavorVersion) {
				v.Version = "..hello"
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "version: version cannot start or end with a space",
		},
		{
			name:        "version starting with ../ does not work",
			prevVersion: new(fixture.FlavorVersion()),
			newVersion: fixture.FlavorVersion(func(v *resource.FlavorVersion) {
				v.Version = "../hello"
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "version: version cannot start or end with a space",
		},
		{
			name:        "version starting with ... does not work",
			prevVersion: new(fixture.FlavorVersion()),
			newVersion: fixture.FlavorVersion(func(v *resource.FlavorVersion) {
				v.Version = "...hello"
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "version: version cannot start or end with a space",
		},
		{
			name:        "version ending with space does not work",
			prevVersion: new(fixture.FlavorVersion()),
			newVersion: fixture.FlavorVersion(func(v *resource.FlavorVersion) {
				v.Version = "hello "
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "version: version cannot start or end with a space",
		},
		{
			name:        "version ending with .. does not work",
			prevVersion: new(fixture.FlavorVersion()),
			newVersion: fixture.FlavorVersion(func(v *resource.FlavorVersion) {
				v.Version = "hello.."
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "version: version cannot start or end with a space",
		},
		{
			name:        "version ending with /.. does not work",
			prevVersion: new(fixture.FlavorVersion()),
			newVersion: fixture.FlavorVersion(func(v *resource.FlavorVersion) {
				v.Version = "hello/.."
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "version: version cannot start or end with a space",
		},
		{
			name:        "version containing /../ does not work",
			prevVersion: new(fixture.FlavorVersion()),
			newVersion: fixture.FlavorVersion(func(v *resource.FlavorVersion) {
				v.Version = "hello/../world"
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "version: version cannot start or end with a space",
		},
		{
			name:        "version containing / does not work",
			prevVersion: new(fixture.FlavorVersion()),
			newVersion: fixture.FlavorVersion(func(v *resource.FlavorVersion) {
				v.Version = "hello/world"
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "version: version cannot start or end with a space",
		},
		{
			name:        "minPlayers has to be greater than 0",
			prevVersion: new(fixture.FlavorVersion()),
			newVersion: fixture.FlavorVersion(func(v *resource.FlavorVersion) {
				v.MinPlayers = 0
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "minPlayers: must be greater than 0",
		},
		{
			name:        "maxPlayers has to be greater than 0",
			prevVersion: new(fixture.FlavorVersion()),
			newVersion: fixture.FlavorVersion(func(v *resource.FlavorVersion) {
				v.MaxPlayers = 0
			}),
			errCode:        codes.InvalidArgument,
			errMsgContains: "maxPlayers: must be greater than 0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				cp  = fixture.NewControlPlane(t)
			)

			cp.Run(t)

			cp.Postgres.CreateChunk(t, &c, fixture.CreateOptions{
				WithFlavors: true,
				WithOwner:   true,
			})

			_, err := cp.Postgres.DB.CreateFlavor(ctx, c.ID, fixture.Flavor())
			require.NoError(t, err)

			cp.AddUserAPIKey(t, &ctx, c.Owner)
			client := cp.ChunkClient(t)

			if tt.prevVersion != nil {
				_, err := client.CreateFlavorVersion(ctx, &chunkv1alpha1.CreateFlavorVersionRequest{
					FlavorId:         c.Flavors[0].ID,
					Version:          tt.prevVersion.Version,
					Hash:             tt.prevVersion.Hash,
					FileHashes:       codec.FileHashSliceToTransport(tt.prevVersion.FileHashes),
					MinecraftVersion: tt.prevVersion.MinecraftVersion,
					MinPlayers:       tt.prevVersion.MinPlayers,
					MaxPlayers:       tt.prevVersion.MaxPlayers,
				})
				require.NoError(t, err)
			}

			version := codec.FlavorVersionToTransport(tt.newVersion)

			resp, err := client.CreateFlavorVersion(ctx, &chunkv1alpha1.CreateFlavorVersionRequest{
				FlavorId:         c.Flavors[0].ID,
				Version:          version.Version,
				Hash:             version.Hash,
				FileHashes:       version.FileHashes,
				MinecraftVersion: version.MinecraftVersion,
				MinPlayers:       version.MinPlayers,
				MaxPlayers:       version.MaxPlayers,
			})

			if err != nil {
				if tt.err != nil {
					if tt.badRequest != nil {
						st, ok := status.FromError(err)
						require.True(t, ok)
						expectedStatus, ok := status.FromError(tt.err)
						require.True(t, ok)
						require.Equal(t, expectedStatus.Code(), st.Code())
						require.Equal(t, expectedStatus.Message(), st.Message())
						require.Len(t, st.Details(), 1)

						badRequest, ok := st.Details()[0].(*errdetails.BadRequest)
						require.True(t, ok)
						require.True(t, proto.Equal(tt.badRequest, badRequest))
					} else {
						require.ErrorIs(t, err, tt.err)
					}
					return
				}

				if tt.errCode != 0 {
					st, ok := status.FromError(err)
					require.True(t, ok)

					require.Equal(t, tt.errCode, st.Code())
					require.Contains(t, st.Message(), tt.errMsgContains)
					return
				}

				require.NoError(t, err)
			}

			expectedVersion := version
			if tt.expectedVersion != nil {
				expectedVersion = codec.FlavorVersionToTransport(*tt.expectedVersion)
			}

			expected := &chunkv1alpha1.CreateFlavorVersionResponse{
				Version: expectedVersion,
			}

			if d := cmp.Diff(
				resp,
				expected,
				protocmp.Transform(),
				test.IgnoredProtoFlavorVersionFields,
			); d != "" {
				t.Fatalf("diff (-want +got):\n%s", d)
			}
		})
	}
}

func TestBuildFlavorVersion(t *testing.T) {
	tests := []struct {
		name string
		prep func(t *testing.T, ctx context.Context, cp fixture.ControlPlane, fakes3 fixture.FakeS3, versionID string)
		err  error
	}{
		{
			name: "works",
			prep: func(t *testing.T, ctx context.Context, cp fixture.ControlPlane, fakes3 fixture.FakeS3, versionID string) {
				fakes3.UploadObject(t, blob.ChangeSetKey(versionID), testdata.FullChangeSetFile)
			},
		},
		{
			// happens when a previous version consisted of the exact same files
			name: "works without tarball when files are already in the blob store",
			prep: func(t *testing.T, ctx context.Context, cp fixture.ControlPlane, fakes3 fixture.FakeS3, versionID string) {
				hashes := seedBlobStore(t, ctx)
				cp.Postgres.InsertBlobs(t, hashes...)
			},
		},
		{
			name: "files not uploaded",
			err:  apierrs.ErrFlavorFilesNotUploaded.GRPCStatus().Err(),
		},
		{
			// versions from before the blob index might carry the flag
			// even though their files never made it to the blob store.
			name: "files not uploaded even though flag is set",
			prep: func(t *testing.T, ctx context.Context, cp fixture.ControlPlane, fakes3 fixture.FakeS3, versionID string) {
				_, err := cp.Postgres.Pool.Exec(ctx, `UPDATE flavor_versions SET files_uploaded = true WHERE id = $1`, versionID)
				require.NoError(t, err)
			},
			err: apierrs.ErrFlavorFilesNotUploaded.GRPCStatus().Err(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				c   = fixture.Chunk(func(tmp *resource.Chunk) {
					tmp.Flavors[0].Versions[0].FileHashes = testdata.ComputeFileHashes(t, "./testdata/serverdata")
				})
				auth = remote.WithAuth(&image.Auth{
					Username: fixture.OCIRegsitryUser,
					Password: fixture.OCIRegistryPass,
				})
			)

			var (
				cp       = fixture.NewControlPlane(t)
				endpoint = fixture.RunRegistry(t)
				fakes3   = fixture.RunFakeS3(t)
			)

			cp.Run(t,
				fixture.WithOCIRegistryEndpoint(endpoint),
				fixture.WithFakeS3Endpoint(fakes3.Endpoint),
			)

			fixture.RunFakeCRI(t)
			fixture.RunCheckpointAPIFixtures(t, fixture.OCIRegsitryUser, fixture.OCIRegistryPass)

			cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)
			cp.Postgres.InsertNode(t)

			flavorVersionID := c.Flavors[0].Versions[0].ID

			if tt.prep != nil {
				tt.prep(t, ctx, cp, fakes3, flavorVersionID)
			}

			// push base image needed for testing

			pusher, err := remote.NewPusher(auth)
			require.NoError(t, err)

			baseImgRef, err := name.ParseReference(fmt.Sprintf("%s/%s", endpoint, fixture.BaseImage))
			require.NoError(t, err)

			err = pusher.Push(ctx, baseImgRef, imgtestdata.Image(t))
			require.NoError(t, err)

			cp.AddUserAPIKey(t, &ctx, c.Owner)
			client := cp.ChunkClient(t)

			_, err = client.BuildFlavorVersion(ctx, &chunkv1alpha1.BuildFlavorVersionRequest{
				FlavorVersionId: flavorVersionID,
			})

			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}

			require.NoError(t, err)

			// verification runs first, so calling build again while it is
			// running must be a no-op and not fail.
			_, err = client.BuildFlavorVersion(ctx, &chunkv1alpha1.BuildFlavorVersionRequest{
				FlavorVersionId: flavorVersionID,
			})
			require.NoError(t, err)

			var (
				timeoutCtx, cancel = context.WithTimeout(ctx, 20*time.Second)
				ticker             = time.NewTicker(200 * time.Millisecond)
			)

			defer cancel()

			for {
				select {
				case <-timeoutCtx.Done():
					t.Fatal("timout reached")
					return
				case <-ticker.C:
					reg, err := name.NewRegistry(endpoint)
					require.NoError(t, err)

					p, err := remote.NewPuller(auth)
					require.NoError(t, err)

					cat, err := p.Catalog(ctx, reg)
					require.NoError(t, err)

					if !slices.Contains(cat, flavorVersionID) {
						continue
					}

					h, err := p.Lister(ctx, reg.Repo(flavorVersionID))
					require.NoError(t, err)

					for h.HasNext() {
						tags, err := h.Next(ctx)
						require.NoError(t, err)

						if slices.Contains(tags.Tags, "checkpoint") && slices.Contains(tags.Tags, "base") {
							actualChunk, err := client.GetChunk(ctx, &chunkv1alpha1.GetChunkRequest{
								Id: c.ID,
							})
							require.NoError(t, err)
							require.True(t, actualChunk.Chunk.Flavors[0].Versions[0].FilesUploaded)

							// every file of the version has to be known to the blob index now
							require.Equal(t, sortedHashes(c.Flavors[0].Versions[0].FileHashes), cp.Postgres.BlobHashes(t))
							return
						}
					}
				}
			}
		})
	}
}

// this is the scenario that used to break image builds: the previous version
// was created, but its files never got uploaded. the client must be told to
// upload everything, not just what changed compared to the previous version.
func TestGetFilesToUpload(t *testing.T) {
	tests := []struct {
		name     string
		seed     func(t *testing.T, cp fixture.ControlPlane, fileHashes []file.Hash)
		expected func(fileHashes []file.Hash) []file.Hash
	}{
		{
			name: "previous version never uploaded, everything has to be uploaded",
			seed: func(t *testing.T, cp fixture.ControlPlane, fileHashes []file.Hash) {},
			expected: func(fileHashes []file.Hash) []file.Hash {
				return fileHashes
			},
		},
		{
			name: "only files missing in the blob store",
			seed: func(t *testing.T, cp fixture.ControlPlane, fileHashes []file.Hash) {
				cp.Postgres.InsertBlobs(t, fileHashes[0].Hash)
			},
			expected: func(fileHashes []file.Hash) []file.Hash {
				return fileHashes[1:]
			},
		},
		{
			name: "everything present",
			seed: func(t *testing.T, cp fixture.ControlPlane, fileHashes []file.Hash) {
				for _, fh := range fileHashes {
					cp.Postgres.InsertBlobs(t, fh.Hash)
				}
			},
			expected: func(fileHashes []file.Hash) []file.Hash {
				return []file.Hash{}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx        = context.Background()
				cp         = fixture.NewControlPlane(t)
				fileHashes = testdata.ComputeFileHashes(t, "./testdata/serverdata")
				c          = fixture.Chunk(func(tmp *resource.Chunk) {
					// Versions[1] is the previous version and stays untouched, so
					// it looks like it was created but never uploaded.
					tmp.Flavors[0].Versions[0].FileHashes = fileHashes
				})
			)

			sort.Slice(fileHashes, func(i, j int) bool {
				return strings.Compare(fileHashes[i].Path, fileHashes[j].Path) < 0
			})

			fixture.RunFakeS3(t)
			cp.Run(t)

			cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)

			tt.seed(t, cp, fileHashes)

			cp.AddUserAPIKey(t, &ctx, c.Owner)
			client := cp.ChunkClient(t)

			resp, err := client.GetFilesToUpload(ctx, &chunkv1alpha1.GetFilesToUploadRequest{
				FlavorVersionId: c.Flavors[0].Versions[0].ID,
			})
			require.NoError(t, err)

			expected := &chunkv1alpha1.GetFilesToUploadResponse{
				Files: codec.FileHashSliceToTransport(tt.expected(fileHashes)),
			}

			if d := cmp.Diff(expected, resp, protocmp.Transform()); d != "" {
				t.Fatalf("diff (-want +got):\n%s", d)
			}
		})
	}
}

func TestGetUploadURLWorks(t *testing.T) {
	var (
		ctx = context.Background()
		cp  = fixture.NewControlPlane(t)
		c   = fixture.Chunk()
	)

	fixture.RunFakeS3(t)
	cp.Run(t)

	cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)

	cp.AddUserAPIKey(t, &ctx, c.Owner)
	client := cp.ChunkClient(t)

	resp, err := client.GetUploadURL(ctx, &chunkv1alpha1.GetUploadURLRequest{
		FlavorVersionId:  c.Flavors[0].Versions[0].ID,
		TarballHash:      "blabla",
		TarballSizeBytes: 10,
	})
	require.NoError(t, err)

	u, err := url.Parse(resp.Url)
	require.NoError(t, err)

	require.Contains(t, u.Query().Get("X-Amz-SignedHeaders"), "content-length")
	require.Contains(t, u.Query().Get("X-Amz-SignedHeaders"), "x-amz-checksum-sha256")
}

func TestGetUploadURLRenews(t *testing.T) {
	tests := []struct {
		name   string
		wait   time.Duration
		equals bool
	}{
		{
			name:   "does not renew",
			wait:   1 * time.Second,
			equals: true,
		},
		{
			name:   "renews",
			wait:   2 * time.Second,
			equals: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				cp  = fixture.NewControlPlane(t)
				c   = fixture.Chunk()
			)

			fixture.RunFakeS3(t)
			cp.Run(t)

			cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)

			cp.AddUserAPIKey(t, &ctx, c.Owner)
			client := cp.ChunkClient(t)

			resp1, err := client.GetUploadURL(ctx, &chunkv1alpha1.GetUploadURLRequest{
				FlavorVersionId: c.Flavors[0].Versions[0].ID,
				TarballHash:     "blabla",
			})
			require.NoError(t, err)

			time.Sleep(tt.wait)

			resp2, err := client.GetUploadURL(ctx, &chunkv1alpha1.GetUploadURLRequest{
				FlavorVersionId: c.Flavors[0].Versions[0].ID,
				TarballHash:     "blabla",
			})
			require.NoError(t, err)

			if tt.equals {
				require.Equal(t, resp1.Url, resp2.Url)
			} else {
				require.NotEqual(t, resp1.Url, resp2.Url)
			}
		})
	}
}

func TestGetUploadURLRequestValidations(t *testing.T) {
	tests := []struct {
		name           string
		req            *chunkv1alpha1.GetUploadURLRequest
		err            error
		errCode        codes.Code
		errMsgContains string
	}{
		{
			name: "invalid flavor version id",
			req: &chunkv1alpha1.GetUploadURLRequest{
				FlavorVersionId: "blabla",
				TarballHash:     "blabla",
			},
			errCode:        codes.InvalidArgument,
			errMsgContains: "flavor_version_id: must be a valid UUID",
		},
		{
			name: "invalid tarball hash",
			req: &chunkv1alpha1.GetUploadURLRequest{
				FlavorVersionId: test.NewUUIDv7(t),
				TarballHash:     "",
			},
			errCode:        codes.InvalidArgument,
			errMsgContains: "tarball_hash: must be at least 1 characters",
		},
		{
			name: "all files already in blob store",
			req:  &chunkv1alpha1.GetUploadURLRequest{},
			err:  apierrs.ErrFlavorFilesUploaded.GRPCStatus().Err(),
		},
		{
			name: "verification running",
			req:  &chunkv1alpha1.GetUploadURLRequest{},
			err:  apierrs.ErrFlavorVersionVerifying.GRPCStatus().Err(),
		},
		{
			name: "changeset file too large",
			req: &chunkv1alpha1.GetUploadURLRequest{
				FlavorVersionId:  test.NewUUIDv7(t),
				TarballHash:      "blabla",
				TarballSizeBytes: fixture.MaxChangeSetTarballSize + 1,
			},
			err: apierrs.ErrChangeSetTarballTooBig.GRPCStatus().Err(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				cp  = fixture.NewControlPlane(t)
				c   = fixture.Chunk()
			)

			fixture.RunFakeS3(t)
			cp.Run(t)

			cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)

			if errors.Is(tt.err, apierrs.ErrFlavorFilesUploaded.GRPCStatus().Err()) {
				for _, fh := range c.Flavors[0].Versions[0].FileHashes {
					cp.Postgres.InsertBlobs(t, fh.Hash)
				}

				tt.req = &chunkv1alpha1.GetUploadURLRequest{
					FlavorVersionId: c.Flavors[0].Versions[0].ID,
					TarballHash:     "blabla",
				}
			}

			if errors.Is(tt.err, apierrs.ErrFlavorVersionVerifying.GRPCStatus().Err()) {
				q := `UPDATE flavor_versions SET build_status = $1 WHERE id = $2`
				_, err := cp.Postgres.Pool.Exec(
					ctx,
					q,
					resource.FlavorVersionBuildStatusFilesVerification,
					c.Flavors[0].Versions[0].ID,
				)
				require.NoError(t, err)

				tt.req = &chunkv1alpha1.GetUploadURLRequest{
					FlavorVersionId: c.Flavors[0].Versions[0].ID,
					TarballHash:     "blabla",
				}
			}

			if errors.Is(tt.err, apierrs.ErrChangeSetTarballTooBig.GRPCStatus().Err()) {
				tt.req.FlavorVersionId = c.Flavors[0].Versions[0].ID
			}

			cp.AddUserAPIKey(t, &ctx, c.Owner)
			client := cp.ChunkClient(t)

			_, err := client.GetUploadURL(ctx, tt.req)

			if tt.errCode != 0 {
				st, ok := status.FromError(err)
				require.True(t, ok)

				require.Equal(t, tt.errCode, st.Code())
				require.Contains(t, st.Message(), tt.errMsgContains)
				return
			}

			require.ErrorIs(t, err, tt.err)
		})
	}
}

func TestGetSupportedMinecraftVersions(t *testing.T) {
	var (
		ctx = context.Background()
		cp  = fixture.NewControlPlane(t)
		u   = fixture.User()
	)

	cp.Run(t)
	cp.Postgres.CreateUser(t, &u)

	cp.AddUserAPIKey(t, &ctx, u)
	client := cp.ChunkClient(t)

	resp, err := client.GetSupportedMinecraftVersions(ctx, &chunkv1alpha1.GetSupportedMinecraftVersionsRequest{})
	require.NoError(t, err)

	if d := cmp.Diff([]string{"1.21.10"}, resp.Versions); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}

func TestUserCannotCreateFlavorVersionForFlavorHeIsNotOwnerOf(t *testing.T) {
	var (
		ctx       = context.Background()
		cp        = fixture.NewControlPlane(t)
		c         = fixture.Chunk()
		otherUser = fixture.OtherIDPUser
	)

	cp.Run(t)
	cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)
	cp.Postgres.CreateUser(t, &otherUser)

	client := cp.ChunkClient(t)
	cp.AddUserAPIKey(t, &ctx, otherUser)

	_, err := client.CreateFlavorVersion(ctx, &chunkv1alpha1.CreateFlavorVersionRequest{
		FlavorId:   c.Flavors[0].ID,
		MinPlayers: 1,
		MaxPlayers: 1,
		//Version:          "",
		//Hash:             "",
		//FileHashes:       nil,
		//MinecraftVersion: "",
	})

	require.ErrorIs(t, err, apierrs.ErrPermissionDenied.GRPCStatus().Err())
}

func TestUserThatIsNotOwnerCannotUpdateChunk(t *testing.T) {
	var (
		ctx       = context.Background()
		cp        = fixture.NewControlPlane(t)
		c         = fixture.Chunk()
		otherUser = fixture.OtherIDPUser
	)

	cp.Run(t)
	cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)
	cp.Postgres.CreateUser(t, &otherUser)

	client := cp.ChunkClient(t)
	cp.AddUserAPIKey(t, &ctx, otherUser)

	_, err := client.UpdateChunk(ctx, &chunkv1alpha1.UpdateChunkRequest{
		Id:          c.ID,
		Name:        "new-name",
		Description: "new-description",
		Tags:        []string{"new-tags"},
	})

	require.ErrorIs(t, err, apierrs.ErrPermissionDenied.GRPCStatus().Err())
}

func TestUserCannotCreateFlavorInChunkWhereHeIsNotOwner(t *testing.T) {
	var (
		ctx       = context.Background()
		cp        = fixture.NewControlPlane(t)
		c         = fixture.Chunk()
		otherUser = fixture.OtherIDPUser
	)

	cp.Run(t)
	cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)
	cp.Postgres.CreateUser(t, &otherUser)

	client := cp.ChunkClient(t)
	cp.AddUserAPIKey(t, &ctx, otherUser)

	_, err := client.CreateFlavor(ctx, &chunkv1alpha1.CreateFlavorRequest{
		ChunkId: c.ID,
		Name:    "some-flavor-name",
	})

	require.ErrorIs(t, err, apierrs.ErrPermissionDenied.GRPCStatus().Err())
}

func TestUserCannotBuildFlavorVersionInFlavorHeIsNotOwnerOf(t *testing.T) {
	var (
		ctx       = context.Background()
		cp        = fixture.NewControlPlane(t)
		c         = fixture.Chunk()
		otherUser = fixture.OtherIDPUser
	)

	cp.Run(t)
	cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)
	cp.Postgres.CreateUser(t, &otherUser)

	client := cp.ChunkClient(t)
	cp.AddUserAPIKey(t, &ctx, otherUser)

	_, err := client.BuildFlavorVersion(ctx, &chunkv1alpha1.BuildFlavorVersionRequest{
		FlavorVersionId: c.Flavors[0].Versions[0].ID,
	})

	require.ErrorIs(t, err, apierrs.ErrPermissionDenied.GRPCStatus().Err())
}

func TestUserCannotGetUploadURLForFlavorVersionWhereHeIsNotOwnerOf(t *testing.T) {
	var (
		ctx       = context.Background()
		cp        = fixture.NewControlPlane(t)
		c         = fixture.Chunk()
		otherUser = fixture.OtherIDPUser
	)

	cp.Run(t)
	cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)
	cp.Postgres.CreateUser(t, &otherUser)

	client := cp.ChunkClient(t)
	cp.AddUserAPIKey(t, &ctx, otherUser)

	_, err := client.GetUploadURL(ctx, &chunkv1alpha1.GetUploadURLRequest{
		FlavorVersionId: c.Flavors[0].Versions[0].ID,
		TarballHash:     "blabla",
	})

	require.ErrorIs(t, err, apierrs.ErrPermissionDenied.GRPCStatus().Err())
}

func TestUploadChunkThumbnailSanityChecks(t *testing.T) {
	tests := []struct {
		name  string
		image []byte
		err   error
	}{
		{
			name:  "invalid thumbnail dimensions too big",
			image: testdata.InvalidThumbnailDimensionsTooBig,
			err:   apierrs.ErrInvalidThumbnailDimensions.GRPCStatus().Err(),
		},
		{
			name:  "invalid thumbnail dimensions too small",
			image: testdata.InvalidThumbnailDimensionsTooSmall,
			err:   apierrs.ErrInvalidThumbnailDimensions.GRPCStatus().Err(),
		},
		{
			name:  "invalid thumbnail wrong format",
			image: testdata.InvalidThumbnailWrongFormat,
			err:   apierrs.ErrInvalidThumbnailFormat.GRPCStatus().Err(),
		},
		{
			name:  "invalid thumbnail size too big",
			image: testdata.InvalidThumbnailSizeTooBig,
			err:   apierrs.ErrInvalidThumbnailSize.GRPCStatus().Err(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				cp  = fixture.NewControlPlane(t)
				c   = fixture.Chunk()
				u   = fixture.User()
			)

			fixture.RunFakeS3(t)
			cp.Run(t)
			cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)
			cp.Postgres.CreateUser(t, &u)
			cp.AddUserAPIKey(t, &ctx, u)

			client := cp.ChunkClient(t)

			_, err := client.UploadThumbnail(ctx, &chunkv1alpha1.UploadThumbnailRequest{
				ChunkId: c.ID,
				Image:   tt.image,
			})

			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestThumbnailActuallyUploadedToS3(t *testing.T) {
	var (
		ctx = context.Background()
		cp  = fixture.NewControlPlane(t)
		c   = fixture.Chunk()
		u   = fixture.User()
	)

	fakes3 := fixture.RunFakeS3(t)
	cp.Run(t)
	cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)
	cp.Postgres.CreateUser(t, &u)
	cp.AddUserAPIKey(t, &ctx, u)

	client := cp.ChunkClient(t)

	_, err := client.UploadThumbnail(ctx, &chunkv1alpha1.UploadThumbnailRequest{
		ChunkId: c.ID,
		Image:   testdata.ValidThumbnail,
	})

	h := fmt.Sprintf("%x", xxh3.Hash(testdata.ValidThumbnail))
	require.NoError(t, err)

	fakes3.RequireObjectExists(t, blob.CASKeyPrefix+"/"+h)
}

func TestThumbnailUploadDoesNotWorkIfChunkIsDeleted(t *testing.T) {
	var (
		ctx = context.Background()
		cp  = fixture.NewControlPlane(t)
		c   = fixture.Chunk(func(tmp *resource.Chunk) {
			tmp.DeletedAt = new(time.Now())
		})
		u = fixture.User()
	)

	cp.Run(t)
	cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)
	cp.Postgres.CreateUser(t, &u)
	cp.AddUserAPIKey(t, &ctx, u)

	client := cp.ChunkClient(t)

	_, err := client.UploadThumbnail(ctx, &chunkv1alpha1.UploadThumbnailRequest{
		ChunkId: c.ID,
		Image:   testdata.ValidThumbnail,
	})

	require.ErrorIs(t, err, apierrs.ErrChunkNotFound.GRPCStatus().Err())
}

func TestAPIDeleteFlavor(t *testing.T) {
	var (
		ctx = context.Background()
		cp  = fixture.NewControlPlane(t)
		c   = fixture.Chunk()
		u   = fixture.User()
	)

	cp.Run(t)
	client := cp.ChunkClient(t)

	cp.Postgres.CreateUser(t, &u)
	cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)
	cp.AddUserAPIKey(t, &ctx, u)

	expected := c
	expected.Flavors = []resource.Flavor{
		c.Flavors[1],
	}

	_, err := client.DeleteFlavor(ctx, &chunkv1alpha1.DeleteFlavorRequest{
		Id: c.Flavors[0].ID,
	})
	require.NoError(t, err)

	resp, err := client.GetChunk(ctx, &chunkv1alpha1.GetChunkRequest{
		Id: c.ID,
	})
	require.NoError(t, err)

	if d := cmp.Diff(
		resp.Chunk,
		codec.ChunkToTransport(expected),
		protocmp.Transform(),
		test.IgnoredProtoChunkFields,
		test.IgnoredProtoFlavorVersionFields,
		test.IgnoredProtoFlavorFields,
		test.IgnoredProtoUserFields,
	); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}

func TestOnlyOwnerCanDeleteFlavor(t *testing.T) {
	var (
		ctx   = context.Background()
		cp    = fixture.NewControlPlane(t)
		c     = fixture.Chunk()
		u     = fixture.User()
		other = fixture.OtherIDPUser
	)

	cp.Run(t)
	client := cp.ChunkClient(t)

	cp.Postgres.CreateUser(t, &u)
	cp.Postgres.CreateUser(t, &other)
	cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)
	cp.AddUserAPIKey(t, &ctx, other)

	_, err := client.DeleteFlavor(ctx, &chunkv1alpha1.DeleteFlavorRequest{
		Id: c.Flavors[0].ID,
	})
	require.ErrorIs(t, err, apierrs.ErrPermissionDenied.GRPCStatus().Err())
}

func TestFlavorInteractionsDontWorkAfterDelete(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		chunkAction    func(context.Context, chunkv1alpha1.ChunkServiceClient, resource.Chunk, resource.Flavor) error
		instanceAction func(context.Context, instancev1alpha1.InstanceServiceClient, resource.Chunk, resource.Flavor) error
	}{
		{
			name: "create flavor version",
			chunkAction: func(
				ctx context.Context,
				c chunkv1alpha1.ChunkServiceClient,
				chunk resource.Chunk,
				flavor resource.Flavor,
			) error {
				_, err := c.CreateFlavorVersion(ctx, &chunkv1alpha1.CreateFlavorVersionRequest{
					FlavorId:         flavor.ID,
					Version:          "v1",
					Hash:             "awdawdawdawd",
					MinecraftVersion: fixture.MinecraftVersion,
					MinPlayers:       1,
					MaxPlayers:       1,
				})
				return err
			},
			err: apierrs.ErrNotFound.GRPCStatus().Err(),
		},
		{
			name: "create new flavor with deleted name",
			chunkAction: func(
				ctx context.Context,
				c chunkv1alpha1.ChunkServiceClient,
				chunk resource.Chunk,
				flavor resource.Flavor,
			) error {
				_, err := c.CreateFlavor(ctx, &chunkv1alpha1.CreateFlavorRequest{
					ChunkId: chunk.ID,
					Name:    flavor.Name,
				})
				return err
			},
			err: apierrs.ErrFlavorNameExists.GRPCStatus().Err(),
		},
		{
			name: "run flavor version",
			instanceAction: func(
				ctx context.Context,
				c instancev1alpha1.InstanceServiceClient,
				chunk resource.Chunk,
				flavor resource.Flavor,
			) error {
				_, err := c.RunFlavorVersion(ctx, &instancev1alpha1.RunFlavorVersionRequest{
					FlavorVersionId: flavor.Versions[0].ID,
				})
				return err
			},
			err: apierrs.ErrNotFound.GRPCStatus().Err(),
		},
		{
			name: "build flavor versions",
			chunkAction: func(
				ctx context.Context,
				c chunkv1alpha1.ChunkServiceClient,
				chunk resource.Chunk,
				flavor resource.Flavor,
			) error {
				_, err := c.BuildFlavorVersion(ctx, &chunkv1alpha1.BuildFlavorVersionRequest{
					FlavorVersionId: flavor.Versions[0].ID,
				})
				return err
			},
			err: apierrs.ErrNotFound.GRPCStatus().Err(),
		},
		{
			name: "get files to upload",
			chunkAction: func(
				ctx context.Context,
				c chunkv1alpha1.ChunkServiceClient,
				chunk resource.Chunk,
				flavor resource.Flavor,
			) error {
				_, err := c.GetFilesToUpload(ctx, &chunkv1alpha1.GetFilesToUploadRequest{
					FlavorVersionId: flavor.Versions[0].ID,
				})
				return err
			},
			err: apierrs.ErrNotFound.GRPCStatus().Err(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				cp  = fixture.NewControlPlane(t)
				c   = fixture.Chunk()
				u   = fixture.User()
			)

			cp.Run(t)

			cp.Postgres.InsertNode(t)
			chunkClient := cp.ChunkClient(t)

			cp.Postgres.CreateUser(t, &u)
			cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)
			cp.AddUserAPIKey(t, &ctx, u)

			insClient := cp.InstanceClient(t)

			_, err := chunkClient.DeleteFlavor(ctx, &chunkv1alpha1.DeleteFlavorRequest{
				Id: c.Flavors[0].ID,
			})
			require.NoError(t, err)

			if tt.chunkAction != nil {
				err = tt.chunkAction(ctx, chunkClient, c, c.Flavors[0])
				require.ErrorIs(t, err, tt.err)
			}

			if tt.instanceAction != nil {
				err = tt.instanceAction(ctx, insClient, c, c.Flavors[0])
				require.ErrorIs(t, err, tt.err)
			}
		})
	}
}

func TestGetChunkReturnsNotFoundIfChunkDeleted(t *testing.T) {
	var (
		ctx = context.Background()
		cp  = fixture.NewControlPlane(t)
		c   = fixture.Chunk(func(tmp *resource.Chunk) {
			tmp.DeletedAt = new(time.Now())
		})
	)

	cp.Run(t)

	cp.AddUserAPIKey(t, &ctx, c.Owner)
	cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)

	client := cp.ChunkClient(t)

	_, err := client.GetChunk(ctx, &chunkv1alpha1.GetChunkRequest{
		Id: c.ID,
	})

	require.ErrorIs(t, err, apierrs.ErrChunkNotFound.GRPCStatus().Err())
}

func TestAPIDeleteChunk(t *testing.T) {
	var (
		ctx = context.Background()
		cp  = fixture.NewControlPlane(t)
		c   = fixture.Chunk()
	)

	cp.Run(t)

	cp.Postgres.InsertNode(t)
	cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)
	cp.AddUserAPIKey(t, &ctx, c.Owner)

	chunkClient := cp.ChunkClient(t)
	insClient := cp.InstanceClient(t)

	_, err := chunkClient.DeleteChunk(ctx, &chunkv1alpha1.DeleteChunkRequest{
		Id: c.ID,
	})
	require.NoError(t, err)

	_, err = chunkClient.GetChunk(ctx, &chunkv1alpha1.GetChunkRequest{
		Id: c.ID,
	})
	require.ErrorIs(t, err, apierrs.ErrChunkNotFound.GRPCStatus().Err(), "get chunk")

	_, err = chunkClient.UpdateChunk(ctx, &chunkv1alpha1.UpdateChunkRequest{
		Id: c.ID,
	})
	require.ErrorIsf(t, err, apierrs.ErrChunkNotFound.GRPCStatus().Err(), "update chunk")

	_, err = chunkClient.UploadThumbnail(ctx, &chunkv1alpha1.UploadThumbnailRequest{
		ChunkId: c.ID,
	})
	require.ErrorIsf(t, err, apierrs.ErrChunkNotFound.GRPCStatus().Err(), "upload thumbnail")

	for _, f := range c.Flavors {
		_, err = chunkClient.CreateFlavorVersion(ctx, &chunkv1alpha1.CreateFlavorVersionRequest{
			FlavorId:         f.ID,
			Version:          f.Versions[0].Version,
			Hash:             f.Versions[0].Hash,
			FileHashes:       codec.FileHashSliceToTransport(f.Versions[0].FileHashes),
			MinecraftVersion: f.Versions[0].MinecraftVersion,
			MaxPlayers:       1,
			MinPlayers:       1,
		})
		require.ErrorIsf(t, err, apierrs.ErrNotFound.GRPCStatus().Err(), "create flavor version (%s)", f.Name)

		_, err = insClient.RunFlavorVersion(ctx, &instancev1alpha1.RunFlavorVersionRequest{
			FlavorVersionId: f.Versions[0].ID,
		})
		require.ErrorIsf(t, err, apierrs.ErrNotFound.GRPCStatus().Err(), "run flavor version (%s)", f.Name)
	}
}

func TestOnlyOwnerCanDeleteChunk(t *testing.T) {
	var (
		ctx   = context.Background()
		cp    = fixture.NewControlPlane(t)
		c     = fixture.Chunk()
		u     = fixture.User()
		other = fixture.OtherIDPUser
	)

	cp.Run(t)
	client := cp.ChunkClient(t)

	cp.Postgres.CreateUser(t, &u)
	cp.Postgres.CreateUser(t, &other)
	cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)
	cp.AddUserAPIKey(t, &ctx, other)

	_, err := client.DeleteChunk(ctx, &chunkv1alpha1.DeleteChunkRequest{
		Id: c.ID,
	})
	require.ErrorIs(t, err, apierrs.ErrPermissionDenied.GRPCStatus().Err())
}

func TestChunkFullyArchived(t *testing.T) {
	var (
		ctx = context.Background()
		c   = fixture.Chunk()
		cp  = fixture.NewControlPlane(t)
	)

	cp.Run(t)
	cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)
	cp.AddUserAPIKey(t, &ctx, c.Owner)

	client := cp.ChunkClient(t)

	_, err := client.DeleteChunk(ctx, &chunkv1alpha1.DeleteChunkRequest{
		Id: c.ID,
	})
	require.NoError(t, err)

	// archive job runs every second
	time.Sleep(5 * time.Second)

	var tmp int

	chunkRow := cp.Postgres.Pool.QueryRow(ctx, `SELECT 1 FROM chunks WHERE id = $1`, c.ID)
	require.ErrorIsf(t, chunkRow.Scan(&tmp), pgx.ErrNoRows, "chunk should be archived")

	for _, f := range c.Flavors {
		flavorRow := cp.Postgres.Pool.QueryRow(ctx, `SELECT 1 FROM flavors WHERE id = $1`, f.ID)
		require.ErrorIsf(t, flavorRow.Scan(&tmp), pgx.ErrNoRows, "flavor should be archived (%s)", f.ID)

		for _, v := range f.Versions {
			versionRow := cp.Postgres.Pool.QueryRow(ctx, `SELECT 1 FROM flavor_versions WHERE id = $1`, v.ID)
			require.ErrorIsf(t, versionRow.Scan(&tmp), pgx.ErrNoRows, "flavor version should be archived (%s)", v.ID)
		}
	}
}

func TestRunFlavorVersionNoSlotsAvailable(t *testing.T) {
	var (
		ctx = context.Background()
		cp  = fixture.NewControlPlane(t)
		c   = fixture.Chunk()
	)

	cp.Run(t)

	cp.Postgres.InsertNode(t)
	cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)

	slots := fixture.Node().Slots
	for i := 0; i < slots; i++ {
		ins := fixture.Instance()
		ins.ID = test.NewUUIDv7(t)
		ins.Chunk = c
		ins.FlavorVersion = c.Flavors[0].Versions[0]
		ins.Owner = c.Owner
		_, err := cp.Postgres.DB.CreateInstance(ctx, ins, fixture.Node().ID)
		require.NoError(t, err)
	}

	cp.AddUserAPIKey(t, &ctx, c.Owner)
	insClient := cp.InstanceClient(t)

	_, err := insClient.RunFlavorVersion(ctx, &instancev1alpha1.RunFlavorVersionRequest{
		FlavorVersionId: c.Flavors[0].Versions[0].ID,
	})

	require.ErrorIs(t, err, apierrs.ErrNoSlotsAvailable.GRPCStatus().Err())
}

func TestFlavorArchived(t *testing.T) {
	var (
		ctx = context.Background()
		c   = fixture.Chunk()
		cp  = fixture.NewControlPlane(t)
	)

	cp.Run(t)
	cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)
	cp.AddUserAPIKey(t, &ctx, c.Owner)

	var (
		client = cp.ChunkClient(t)
		f      = c.Flavors[0]
	)

	_, err := client.DeleteFlavor(ctx, &chunkv1alpha1.DeleteFlavorRequest{
		Id: f.ID,
	})
	require.NoError(t, err)

	time.Sleep(5 * time.Second)

	_, err = client.GetFlavor(ctx, &chunkv1alpha1.GetFlavorRequest{
		Id: f.ID,
	})
	require.ErrorIs(t, err, apierrs.ErrNotFound.GRPCStatus().Err())

	var tmp int

	flavorRow := cp.Postgres.Pool.QueryRow(ctx, `SELECT 1 FROM flavors WHERE id = $1`, f.ID)
	require.ErrorIsf(t, flavorRow.Scan(&tmp), pgx.ErrNoRows, "flavor should be archived (%s)", f.ID)

	for _, v := range f.Versions {
		versionRow := cp.Postgres.Pool.QueryRow(ctx, `SELECT 1 FROM flavor_versions WHERE id = $1`, v.ID)
		require.ErrorIsf(t, versionRow.Scan(&tmp), pgx.ErrNoRows, "flavor version should be archived (%s)", v.ID)
	}

	chunkRow := cp.Postgres.Pool.QueryRow(ctx, `SELECT 1 FROM chunks WHERE id = $1`, c.ID)

	err = chunkRow.Scan(&tmp)
	require.NoError(t, err)

	require.Equal(t, 1, tmp, "chunk should not be archived")
}

func TestGetFlavor(t *testing.T) {
	tests := []struct {
		name           string
		flavorID       string
		errCode        codes.Code
		errMsgContains string
		err            error
	}{
		{
			name: "works",
		},
		{
			name:           "not found",
			flavorID:       test.NewUUIDv7(t),
			errCode:        codes.NotFound,
			errMsgContains: "resource does not exist",
		},
		{
			name:           "invalid id",
			flavorID:       "invalid",
			errCode:        codes.InvalidArgument,
			errMsgContains: "id: must be a valid UUID",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				cp  = fixture.NewControlPlane(t)
				c   = fixture.Chunk(func(tmp *resource.Chunk) {
					tmp.Flavors = []resource.Flavor{
						fixture.Flavor(func(f *resource.Flavor) {
							f.Versions = []resource.FlavorVersion{
								fixture.FlavorVersion(),
							}
						}),
					}
				})
			)

			cp.Run(t)

			if tt.flavorID == "" {
				cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)
				tt.flavorID = c.Flavors[0].ID
			} else {
				cp.Postgres.CreateUser(t, &c.Owner)
			}

			cp.AddUserAPIKey(t, &ctx, c.Owner)
			client := cp.ChunkClient(t)

			resp, err := client.GetFlavor(ctx, &chunkv1alpha1.GetFlavorRequest{
				Id: tt.flavorID,
			})

			if tt.errCode != 0 {
				st, ok := status.FromError(err)
				require.True(t, ok)

				require.Equal(t, tt.errCode, st.Code())
				require.Contains(t, st.Message(), tt.errMsgContains)
				return
			}

			require.NoError(t, err)

			c.Flavors[0].Versions[0].FileHashes = nil // file hashes are not sent when getting a flavor

			if d := cmp.Diff(
				codec.FlavorToTransport(c.Flavors[0]),
				resp.GetFlavor(),
				protocmp.Transform(),
				test.IgnoredProtoChunkFields,
				test.IgnoredProtoFlavorFields,
				test.IgnoredProtoFlavorVersionFields,
				test.IgnoredProtoUserFields,
			); d != "" {
				t.Fatalf("diff (-want +got):\n%s", d)
			}
		})
	}
}

// walks through a publish the way the cli does it, including the case that
// used to break builds: a version whose files were never uploaded, followed
// by a version that only ships what changed compared to it.
func TestPublishFlowEndToEnd(t *testing.T) {
	var (
		ctx        = context.Background()
		fileHashes = testdata.ComputeFileHashes(t, "./testdata/serverdata")
		c          = fixture.Chunk()
		auth       = remote.WithAuth(&image.Auth{
			Username: fixture.OCIRegsitryUser,
			Password: fixture.OCIRegistryPass,
		})
	)

	sort.Slice(fileHashes, func(i, j int) bool {
		return strings.Compare(fileHashes[i].Path, fileHashes[j].Path) < 0
	})

	var (
		cp       = fixture.NewControlPlane(t)
		endpoint = fixture.RunRegistry(t)
		fakes3   = fixture.RunFakeS3(t)
	)

	cp.Run(t,
		fixture.WithOCIRegistryEndpoint(endpoint),
		fixture.WithFakeS3Endpoint(fakes3.Endpoint),
	)

	fixture.RunFakeCRI(t)
	fixture.RunCheckpointAPIFixtures(t, fixture.OCIRegsitryUser, fixture.OCIRegistryPass)

	cp.Postgres.CreateChunk(t, &c, fixture.CreateOptions{
		WithFlavors: true,
		WithOwner:   true,
	})
	cp.Postgres.InsertNode(t)

	pusher, err := remote.NewPusher(auth)
	require.NoError(t, err)

	baseImgRef, err := name.ParseReference(fmt.Sprintf("%s/%s", endpoint, fixture.BaseImage))
	require.NoError(t, err)

	err = pusher.Push(ctx, baseImgRef, imgtestdata.Image(t))
	require.NoError(t, err)

	cp.AddUserAPIKey(t, &ctx, c.Owner)
	client := cp.ChunkClient(t)

	flavorID := c.Flavors[0].ID

	// v1 gets created, but the files are never uploaded.

	createVersion(t, ctx, client, flavorID, "v1", fileHashes)

	// v2 consists of the same files. an old cli would diff against v1,
	// find nothing to upload and the image build would blow up.

	v2 := createVersion(t, ctx, client, flavorID, "v2", fileHashes)

	requireFilesToUpload(t, ctx, client, v2, fileHashes)

	// pretend to be that old cli and only upload a part of the files.

	partial := tarFiles(t, "./testdata/serverdata", "./testdata/serverdata/server.properties")
	uploadChangeSet(t, ctx, client, v2, partial)

	_, err = client.BuildFlavorVersion(ctx, &chunkv1alpha1.BuildFlavorVersionRequest{FlavorVersionId: v2})
	require.NoError(t, err)

	waitForAPIBuildStatus(t, ctx, client, c.ID, v2, chunkv1alpha1.BuildStatus_FILES_VERIFICATION_FAILED)

	// what was in the partial tarball is in the blob store now,
	// so only the rest is expected to be uploaded.

	var serverProps file.Hash
	for _, fh := range fileHashes {
		if fh.Path == "server.properties" {
			serverProps = fh
		}
	}
	require.NotEmpty(t, serverProps.Hash)

	requireFilesToUpload(t, ctx, client, v2, withoutHash(fileHashes, serverProps.Hash))

	uploadChangeSet(t, ctx, client, v2, testdata.FullChangeSetFile)

	_, err = client.BuildFlavorVersion(ctx, &chunkv1alpha1.BuildFlavorVersionRequest{FlavorVersionId: v2})
	require.NoError(t, err)

	waitForAPIBuildStatus(t, ctx, client, c.ID, v2, chunkv1alpha1.BuildStatus_COMPLETED)
	requireFilesToUpload(t, ctx, client, v2, []file.Hash{})

	// v3 changes a single file. only that one has to be uploaded.

	var (
		v3Dir  = t.TempDir()
		v3File = filepath.Join(v3Dir, "server.properties")
	)

	require.NoError(t, os.WriteFile(v3File, []byte("motd=changed\n"), 0644))

	changed := testdata.ComputeFileHashes(t, v3Dir)[0]
	require.Equal(t, "server.properties", changed.Path)

	v3Hashes := make([]file.Hash, 0, len(fileHashes))
	for _, fh := range fileHashes {
		if fh.Path == "server.properties" {
			v3Hashes = append(v3Hashes, changed)
			continue
		}
		v3Hashes = append(v3Hashes, fh)
	}

	v3 := createVersion(t, ctx, client, flavorID, "v3", v3Hashes)

	requireFilesToUpload(t, ctx, client, v3, []file.Hash{changed})

	uploadChangeSet(t, ctx, client, v3, tarFiles(t, v3Dir, v3File))

	_, err = client.BuildFlavorVersion(ctx, &chunkv1alpha1.BuildFlavorVersionRequest{FlavorVersionId: v3})
	require.NoError(t, err)

	waitForAPIBuildStatus(t, ctx, client, c.ID, v3, chunkv1alpha1.BuildStatus_COMPLETED)

	// the image has to contain every file, not just the uploaded one
	checkImage(t, ctx, auth, endpoint, v3, v3Hashes)
}

func createVersion(
	t *testing.T,
	ctx context.Context,
	client chunkv1alpha1.ChunkServiceClient,
	flavorID string,
	version string,
	fileHashes []file.Hash,
) string {
	v := codec.FlavorVersionToTransport(fixture.FlavorVersion(func(v *resource.FlavorVersion) {
		v.Version = version
		v.FileHashes = fileHashes
	}))

	resp, err := client.CreateFlavorVersion(ctx, &chunkv1alpha1.CreateFlavorVersionRequest{
		FlavorId:         flavorID,
		Version:          v.Version,
		Hash:             v.Hash,
		FileHashes:       v.FileHashes,
		MinecraftVersion: v.MinecraftVersion,
		MinPlayers:       v.MinPlayers,
		MaxPlayers:       v.MaxPlayers,
	})
	require.NoError(t, err)

	return resp.Version.Id
}

func requireFilesToUpload(
	t *testing.T,
	ctx context.Context,
	client chunkv1alpha1.ChunkServiceClient,
	versionID string,
	expected []file.Hash,
) {
	resp, err := client.GetFilesToUpload(ctx, &chunkv1alpha1.GetFilesToUploadRequest{
		FlavorVersionId: versionID,
	})
	require.NoError(t, err)

	want := &chunkv1alpha1.GetFilesToUploadResponse{
		Files: codec.FileHashSliceToTransport(expected),
	}

	if d := cmp.Diff(want, resp, protocmp.Transform()); d != "" {
		t.Fatalf("files to upload mismatch (-want +got):\n%s", d)
	}
}

func withoutHash(fileHashes []file.Hash, hash string) []file.Hash {
	ret := make([]file.Hash, 0, len(fileHashes))
	for _, fh := range fileHashes {
		if fh.Hash == hash {
			continue
		}
		ret = append(ret, fh)
	}
	return ret
}

func tarFiles(t *testing.T, rootDir string, paths ...string) []byte {
	files := make([]*os.File, 0, len(paths))
	for _, p := range paths {
		f, err := os.Open(p)
		require.NoError(t, err)

		defer f.Close()

		files = append(files, f)
	}

	dest := filepath.Join(t.TempDir(), "changeset.tar.gz")
	require.NoError(t, tarhelper.TarFiles(rootDir, files, dest))

	data, err := os.ReadFile(dest)
	require.NoError(t, err)

	return data
}

// uploads the tarball the same way the cli does: presigned url,
// content length and sha256 checksum header.
func uploadChangeSet(
	t *testing.T,
	ctx context.Context,
	client chunkv1alpha1.ChunkServiceClient,
	versionID string,
	tarball []byte,
) {
	var (
		digest = sha256.Sum256(tarball)
		hash   = base64.StdEncoding.EncodeToString(digest[:])
	)

	resp, err := client.GetUploadURL(ctx, &chunkv1alpha1.GetUploadURLRequest{
		FlavorVersionId:  versionID,
		TarballHash:      hash,
		TarballSizeBytes: uint64(len(tarball)),
	})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, resp.Url, bytes.NewReader(tarball))
	require.NoError(t, err)

	req.ContentLength = int64(len(tarball))
	req.Header.Set("x-amz-checksum-sha256", hash)

	uploadResp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer uploadResp.Body.Close()

	body, _ := io.ReadAll(uploadResp.Body)
	require.Equal(t, http.StatusOK, uploadResp.StatusCode, string(body))
}

func waitForAPIBuildStatus(
	t *testing.T,
	ctx context.Context,
	client chunkv1alpha1.ChunkServiceClient,
	chunkID string,
	versionID string,
	status chunkv1alpha1.BuildStatus,
) {
	var (
		timeoutCtx, cancel = context.WithTimeout(ctx, 60*time.Second)
		ticker             = time.NewTicker(500 * time.Millisecond)
	)

	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			t.Fatalf("timeout reached waiting for build status %s of version %s", status, versionID)
			return
		case <-ticker.C:
			resp, err := client.GetChunk(ctx, &chunkv1alpha1.GetChunkRequest{Id: chunkID})
			require.NoError(t, err)

			for _, f := range resp.Chunk.Flavors {
				for _, v := range f.Versions {
					if v.Id == versionID && v.BuildStatus == status {
						return
					}
				}
			}
		}
	}
}
