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
	"testing"

	"github.com/google/go-cmp/cmp"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/internal/ptr"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestCreateFlavor(t *testing.T) {
	c := fixture.Chunk()
	tests := []struct {
		name  string
		req   *chunkv1alpha1.CreateFlavorRequest
		other *chunk.Flavor
		err   error
	}{
		{
			name: "works",
			req: &chunkv1alpha1.CreateFlavorRequest{
				ChunkId: &c.ID,
				Name:    ptr.Pointer(fixture.Flavor().Name),
			},
		},
		{
			name: "flavor already exists",
			req: &chunkv1alpha1.CreateFlavorRequest{
				ChunkId: &c.ID,
				Name:    ptr.Pointer(fixture.Flavor().Name),
			},
			other: ptr.Pointer(fixture.Flavor()),
			err:   chunk.ErrFlavorNameExists,
		},
		{
			name: "invalid chunk id",
			req: &chunkv1alpha1.CreateFlavorRequest{
				Name: ptr.Pointer("adawdaw"),
			},
			err: chunk.ErrInvalidChunkID,
		},
		{
			name: "invalid flavor name",
			req: &chunkv1alpha1.CreateFlavorRequest{
				ChunkId: &c.ID,
			},
			err: chunk.ErrInvalidChunkID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				pg  = fixture.NewPostgres()
			)

			fixture.RunControlPlane(t, pg)

			_, err := pg.DB.CreateChunk(ctx, c)
			require.NoError(t, err)

			if tt.other != nil {
				_, err := pg.DB.CreateFlavor(ctx, c.ID, *tt.other)
				require.NoError(t, err)
			}

			conn, err := grpc.NewClient(
				fixture.ControlPlaneAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			client := chunkv1alpha1.NewChunkServiceClient(conn)

			resp, err := client.CreateFlavor(ctx, tt.req)

			if tt.err != nil {
				require.ErrorAs(t, err, &tt.err)
				return
			}

			require.NoError(t, err)
			expected := &chunkv1alpha1.Flavor{
				Id:   tt.req.ChunkId,
				Name: tt.req.Name,
			}

			if d := cmp.Diff(
				resp.GetFlavor(),
				expected,
				protocmp.Transform(),
				protocmp.IgnoreFields(&chunkv1alpha1.Flavor{}, "id", "updated_at", "created_at"),
			); d != "" {
				t.Fatalf("CreateFlavorResponse mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestCreateFlavorVersion(t *testing.T) {
	var (
		c      = fixture.Chunk()
		flavor = fixture.Chunk().Flavors[0]
	)

	tests := []struct {
		name        string
		prevVersion *chunk.FlavorVersion
		newVersion  chunk.FlavorVersion
		diff        chunk.FlavorVersionDiff
		err         error
	}{
		{
			name: "create initial version",
			newVersion: fixture.FlavorVersion(t, func(f *chunk.FlavorVersion) {
				f.Flavor = flavor
			}),
			diff: chunk.FlavorVersionDiff{
				Added: fixture.FlavorVersion(t).FileHashes,
			},
		},
		{
			name: "create second version with changed files",
			prevVersion: ptr.Pointer(fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
				v.Flavor = flavor
			})),
			newVersion: fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
				v.Flavor = flavor
				v.Version = "v2"
				v.FileHashes = []chunk.FileHash{
					// plugins/myplugin/config.json not present -> its removed
					{
						Path: "paper.yml", // unchanged
						Hash: "pppppppppppppppp",
					},
					{
						Path: "server.properties", // changed
						Hash: "cccccccccccccccc",
					},
					{
						Path: "plugins/myplugin.jar", // added
						Hash: "yyyyyyyyyyyyyyyy",
					},
				}
			}),
			diff: chunk.FlavorVersionDiff{
				Added: []chunk.FileHash{
					{
						Path: "plugins/myplugin.jar",
						Hash: "yyyyyyyyyyyyyyyy",
					},
				},
				Changed: []chunk.FileHash{
					{
						Path: "server.properties",
						Hash: "cccccccccccccccc",
					},
				},
				Removed: []chunk.FileHash{
					{
						Path: "plugins/myplugin/config.json",
						Hash: "cooooooooooooooo",
					},
				},
			},
		},
		{
			name: "version already exists",
			prevVersion: ptr.Pointer(fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
				v.Flavor = flavor
			})),
			newVersion: fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
				v.Flavor = flavor
			}),
			err: chunk.ErrFlavorVersionExists,
		},
		{
			name: "version mismatch",
			prevVersion: ptr.Pointer(fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
				v.Flavor = flavor
			})),
			newVersion: fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
				v.Flavor = flavor
				v.Version = "v2"
				v.Hash = "wrong-hash"
			}),
			err: chunk.ErrFlavorVersionHashMismatch,
		},
		{
			name: "duplicate version",
			prevVersion: ptr.Pointer(fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
				v.Flavor = flavor
			})),
			newVersion: fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
				v.Flavor = flavor
				v.Version = "v2"
			}),
			err: chunk.ErrFlavorVersionDuplicate{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				pg  = fixture.NewPostgres()
			)

			fixture.RunControlPlane(t, pg)

			_, err := pg.DB.CreateChunk(ctx, c)
			require.NoError(t, err)

			_, err = pg.DB.CreateFlavor(ctx, c.ID, flavor)
			require.NoError(t, err)

			conn, err := grpc.NewClient(
				fixture.ControlPlaneAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			client := chunkv1alpha1.NewChunkServiceClient(conn)

			if tt.prevVersion != nil {
				_, err := client.CreateFlavorVersion(ctx, &chunkv1alpha1.CreateFlavorVersionRequest{
					Version: chunk.FlavorVersionToTransport(*tt.prevVersion),
				})
				require.NoError(t, err)
			}

			version := chunk.FlavorVersionToTransport(tt.newVersion)

			resp, err := client.CreateFlavorVersion(ctx, &chunkv1alpha1.CreateFlavorVersionRequest{
				Version: version,
			})

			if err != nil {
				if tt.err != nil {
					require.ErrorAs(t, err, &tt.err)
					return
				}
				require.NoError(t, err)
			}

			expected := &chunkv1alpha1.CreateFlavorVersionResponse{
				Version:      version,
				AddedFiles:   chunk.FileHashSliceToTransport(tt.diff.Added),
				ChangedFiles: chunk.FileHashSliceToTransport(tt.diff.Changed),
				RemovedFiles: chunk.FileHashSliceToTransport(tt.diff.Removed),
			}

			if d := cmp.Diff(
				resp,
				expected,
				protocmp.Transform(),
				protocmp.IgnoreFields(&chunkv1alpha1.FlavorVersion{}, "id", "created_at"),
			); d != "" {
				t.Fatalf("CreateFlavorVersionResponse mismatch (-want +got):\n%s", d)
			}
		})
	}
}
