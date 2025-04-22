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
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/internal/ptr"
	"github.com/spacechunks/explorer/test"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestAPICreateChunk(t *testing.T) {
	tests := []struct {
		name     string
		expected chunk.Chunk
		err      error
	}{
		{
			name: "works",
			expected: fixture.Chunk(func(c *chunk.Chunk) {
				c.Flavors = nil
			}),
		},
		{
			name: "name too long",
			expected: fixture.Chunk(func(c *chunk.Chunk) {
				c.Name = strings.Repeat("a", chunk.MaxChunkNameChars+1)
			}),
			err: chunk.ErrNameTooLong,
		},
		{
			name: "description too long",
			expected: fixture.Chunk(func(c *chunk.Chunk) {
				c.Description = strings.Repeat("a", chunk.MaxChunkDescriptionChars+1)
			}),
			err: chunk.ErrDescriptionTooLong,
		},
		{
			name: "too many tags",
			expected: fixture.Chunk(func(c *chunk.Chunk) {
				c.Tags = slices.Repeat([]string{"a"}, chunk.MaxChunkTags+1)
			}),
			err: chunk.ErrTooManyTags,
		},
		{
			name: "invalid name",
			expected: fixture.Chunk(func(c *chunk.Chunk) {
				c.Name = ""
			}),
			err: chunk.ErrInvalidName,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				pg  = fixture.NewPostgres()
			)

			fixture.RunControlPlane(t, pg)

			conn, err := grpc.NewClient(
				fixture.ControlPlaneAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			client := chunkv1alpha1.NewChunkServiceClient(conn)

			resp, err := client.CreateChunk(ctx, &chunkv1alpha1.CreateChunkRequest{
				Name:        tt.expected.Name,
				Description: tt.expected.Description,
				Tags:        tt.expected.Tags,
			})

			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}

			require.NoError(t, err)

			if d := cmp.Diff(
				chunk.ChunkToTransport(tt.expected),
				resp.GetChunk(),
				protocmp.Transform(),
				test.IgnoredProtoChunkFields,
			); d != "" {
				t.Fatalf("diff (-want +got):\n%s", d)
			}
		})
	}
}

func TestGetChunk(t *testing.T) {
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
			err:  chunk.ErrChunkNotFound,
		},
		{
			name: "invalid id",
			err:  chunk.ErrInvalidChunkID,
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

			if tt.create {
				pg.CreateChunk(t, &c)
			}

			conn, err := grpc.NewClient(
				fixture.ControlPlaneAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			client := chunkv1alpha1.NewChunkServiceClient(conn)

			if tt.err == chunk.ErrChunkNotFound {
				c.ID = test.NewUUIDv7(t)
			}

			if tt.err == chunk.ErrInvalidChunkID {
				c.ID = ""
			}

			resp, err := client.GetChunk(ctx, &chunkv1alpha1.GetChunkRequest{
				Id: c.ID,
			})

			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}

			require.NoError(t, err)

			if d := cmp.Diff(
				chunk.ChunkToTransport(c),
				resp.GetChunk(),
				protocmp.Transform(),
				test.IgnoredProtoChunkFields,
				test.IgnoredProtoFlavorFields,
			); d != "" {
				t.Fatalf("diff (-want +got):\n%s", d)
			}
		})
	}
}

func TestListChunks(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
	)

	fixture.RunControlPlane(t, pg)

	chunks := []chunk.Chunk{
		fixture.Chunk(func(c *chunk.Chunk) {
			c.ID = test.NewUUIDv7(t)
			c.Flavors = []chunk.Flavor{
				fixture.Flavor(func(f *chunk.Flavor) {
					f.ID = test.NewUUIDv7(t)
					f.Name = "ddddawq31423452"
				}),
			}
		}),
		fixture.Chunk(func(c *chunk.Chunk) {
			c.ID = test.NewUUIDv7(t)
			c.Flavors = []chunk.Flavor{
				fixture.Flavor(func(f *chunk.Flavor) {
					f.ID = test.NewUUIDv7(t)
					f.Name = "dawdawdawd"
				}),
			}
		}),
	}

	for i := range chunks {
		pg.CreateChunk(t, &chunks[i])
	}

	conn, err := grpc.NewClient(
		fixture.ControlPlaneAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	client := chunkv1alpha1.NewChunkServiceClient(conn)

	resp, err := client.ListChunks(ctx, &chunkv1alpha1.ListChunksRequest{})
	require.NoError(t, err)

	expected := make([]*chunkv1alpha1.Chunk, 0, len(chunks))
	for _, c := range chunks {
		expected = append(expected, chunk.ChunkToTransport(c))
	}

	if d := cmp.Diff(
		expected,
		resp.GetChunks(),
		protocmp.Transform(),
		test.IgnoredProtoChunkFields,
		test.IgnoredProtoFlavorFields,
	); d != "" {
		t.Fatalf("diff (-want +got):\n%s", d)
	}
}

func TestUpdateChunk(t *testing.T) {
	tests := []struct {
		name string
		req  *chunkv1alpha1.UpdateChunkRequest
		err  error
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
			err: chunk.ErrChunkNotFound,
		},
		{
			name: "name too long",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Name: strings.Repeat("a", chunk.MaxChunkNameChars+1),
			},
			err: chunk.ErrNameTooLong,
		},
		{
			name: "description too long",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Description: strings.Repeat("a", chunk.MaxChunkDescriptionChars+1),
			},
			err: chunk.ErrDescriptionTooLong,
		},
		{
			name: "too many tags",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Tags: slices.Repeat([]string{"a"}, chunk.MaxChunkTags+1),
			},
			err: chunk.ErrTooManyTags,
		},
		{
			name: "invalid chunk id",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Id: "invalid",
			},
			err: chunk.ErrInvalidChunkID,
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

			pg.CreateChunk(t, &c)

			conn, err := grpc.NewClient(
				fixture.ControlPlaneAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			client := chunkv1alpha1.NewChunkServiceClient(conn)

			if tt.req.Id == "" {
				tt.req.Id = c.ID
			}

			resp, err := client.UpdateChunk(ctx, tt.req)

			if tt.err != nil {
				require.ErrorAs(t, err, &tt.err)
				return
			}

			require.NoError(t, err)

			expected := chunk.ChunkToTransport(c)

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
				resp.GetChunk(),
				expected,
				protocmp.Transform(),
				test.IgnoredProtoChunkFields,
				test.IgnoredProtoFlavorFields,
			); d != "" {
				t.Fatalf("diff (-want +got):\n%s", d)
			}
		})
	}
}

func TestCreateFlavor(t *testing.T) {
	c := fixture.Chunk()
	tests := []struct {
		name       string
		flavorName string
		other      *chunk.Flavor
		err        error
	}{
		{
			name:       "works",
			flavorName: fixture.Flavor().Name,
		},
		{
			name:       "flavor already exists",
			flavorName: fixture.Flavor().Name,
			other:      ptr.Pointer(fixture.Flavor()),
			err:        chunk.ErrFlavorNameExists,
		},
		{
			name:       "invalid chunk id",
			flavorName: "awdawdawd",
			err:        chunk.ErrInvalidChunkID,
		},
		{
			name: "invalid flavor name",
			err:  chunk.ErrInvalidName,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				pg  = fixture.NewPostgres()
			)

			fixture.RunControlPlane(t, pg)

			pg.CreateChunk(t, &c)

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

			resp, err := client.CreateFlavor(ctx, &chunkv1alpha1.CreateFlavorRequest{
				ChunkId: c.ID,
				Name:    tt.flavorName,
			})

			if tt.err != nil {
				require.ErrorAs(t, err, &tt.err)
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

func TestListFlavors(t *testing.T) {
	tests := []struct {
		name    string
		chunkID string
		err     error
	}{
		{
			name: "works",
		},
		{
			name: "invalid chunk id",
			err:  chunk.ErrInvalidChunkID,
		},
		{
			name: "chunk not found",
			err:  chunk.ErrChunkNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				pg  = fixture.NewPostgres()
				c   = fixture.Chunk(func(c *chunk.Chunk) {
					c.Flavors = []chunk.Flavor{
						fixture.Flavor(func(f *chunk.Flavor) {
							f.Name = "f1"
						}),
						fixture.Flavor(func(f *chunk.Flavor) {
							f.Name = "f2"
						}),
					}
				})
			)

			fixture.RunControlPlane(t, pg)

			var expected []*chunkv1alpha1.Flavor

			pg.CreateChunk(t, &c)
			for _, f := range c.Flavors {
				expected = append(expected, chunk.FlavorToTransport(f))
			}

			conn, err := grpc.NewClient(
				fixture.ControlPlaneAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			client := chunkv1alpha1.NewChunkServiceClient(conn)

			if tt.err == nil {
				tt.chunkID = c.ID
			}

			resp, err := client.ListFlavors(ctx, &chunkv1alpha1.ListFlavorsRequest{
				ChunkId: tt.chunkID,
			})

			if tt.err != nil {
				require.ErrorAs(t, err, &tt.err)
				return
			}

			require.NoError(t, err)

			if d := cmp.Diff(
				resp.GetFlavors(),
				expected,
				protocmp.Transform(),
				test.IgnoredProtoFlavorFields,
			); d != "" {
				t.Fatalf("mismatch (-want +got):\n%s", d)
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
			name: "version hash mismatch",
			prevVersion: ptr.Pointer(fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
				v.Flavor = flavor
			})),
			newVersion: fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
				v.Flavor = flavor
				v.Version = "v2"
				v.Hash = "wrong-hash"
			}),
			err: chunk.ErrHashMismatch,
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

			pg.CreateChunk(t, &c)

			conn, err := grpc.NewClient(
				fixture.ControlPlaneAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			client := chunkv1alpha1.NewChunkServiceClient(conn)

			if tt.prevVersion != nil {
				tt.prevVersion.Flavor.ID = c.Flavors[0].ID
				_, err := client.CreateFlavorVersion(ctx, &chunkv1alpha1.CreateFlavorVersionRequest{
					Version: chunk.FlavorVersionToTransport(*tt.prevVersion),
				})
				require.NoError(t, err)
			}

			tt.newVersion.Flavor.ID = c.Flavors[0].ID
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
				test.IgnoredProtoFlavorVersionFields,
			); d != "" {
				t.Fatalf("diff (-want +got):\n%s", d)
			}
		})
	}
}

func TestSaveFlavorFiles(t *testing.T) {
	files := []chunk.File{
		{
			Path: "plugins/testadata1",
			Data: []byte("ugede ishde"),
		},
		{
			Path: "plugins/testadata2",
			Data: []byte("hello world"),
		},
	}

	tests := []struct {
		name    string
		files   []chunk.File
		version chunk.FlavorVersion
		err     error
	}{
		{
			name:  "works",
			files: files,
			version: fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
				v.FileHashes = []chunk.FileHash{
					{
						Path: "plugins/testadata1",
						Hash: "1f47515caccc8b7c",
					},
					{
						Path: "plugins/testadata2",
						Hash: "d447b1ea40e6988b",
					},
				}
			}),
		},
		{
			name:  "hash mismatch - file missing",
			files: files,
			version: fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
				v.FileHashes = []chunk.FileHash{
					{
						Path: "plugins/testadata1",
						Hash: "1f47515caccc8b7c",
					},
				}
			}),
			err: chunk.ErrHashMismatch,
		},
		{
			name:  "hash mismatch - unexpected file",
			files: files,
			version: fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
				v.FileHashes = []chunk.FileHash{
					{
						Path: "plugins/testadata1",
						Hash: "1f47515caccc8b7c",
					},
					{
						Path: "plugins/testadata2",
						Hash: "d447b1ea40e6988b",
					},
					{
						Path: "plugins/testadata3",
						Hash: "d447b1ea40e6988b",
					},
				}
			}),
			err: chunk.ErrHashMismatch,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				pg  = fixture.NewPostgres()
			)

			fixture.RunControlPlane(t, pg)

			c := fixture.Chunk()
			pg.CreateChunk(t, &c)

			conn, err := grpc.NewClient(
				fixture.ControlPlaneAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			client := chunkv1alpha1.NewChunkServiceClient(conn)

			tt.version.Flavor.ID = c.Flavors[0].ID

			resp, err := client.CreateFlavorVersion(ctx, &chunkv1alpha1.CreateFlavorVersionRequest{
				Version: chunk.FlavorVersionToTransport(tt.version),
			})
			require.NoError(t, err)

			files := make([]*chunkv1alpha1.File, 0, len(tt.files))
			for _, file := range tt.files {
				files = append(files, &chunkv1alpha1.File{
					Path: file.Path,
					Data: file.Data,
				})
			}

			_, err = client.SaveFlavorFiles(ctx, &chunkv1alpha1.SaveFlavorFilesRequest{
				FlavorVersionId: resp.GetVersion().Id,
				Files:           files,
			})

			if err != nil {
				if tt.err != nil {
					require.ErrorIs(t, err, tt.err)
					return
				}
			}

			require.NoError(t, err)
		})
	}
}

func TestSaveFlavorFilesAlreadyUploaded(t *testing.T) {
	var (
		ctx   = context.Background()
		pg    = fixture.NewPostgres()
		files = []chunk.File{
			{
				Path: "plugins/testadata1",
				Data: []byte("ugede ishde"),
			},
			{
				Path: "plugins/testadata2",
				Data: []byte("hello world"),
			},
		}

		flavorVersion = fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
			v.FileHashes = []chunk.FileHash{
				{
					Path: "plugins/testadata1",
					Hash: "1f47515caccc8b7c",
				},
				{
					Path: "plugins/testadata2",
					Hash: "d447b1ea40e6988b",
				},
			}
		})
	)

	fixture.RunControlPlane(t, pg)

	c := fixture.Chunk()
	pg.CreateChunk(t, &c)

	conn, err := grpc.NewClient(
		fixture.ControlPlaneAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	client := chunkv1alpha1.NewChunkServiceClient(conn)

	flavorVersion.Flavor.ID = c.Flavors[0].ID

	resp, err := client.CreateFlavorVersion(ctx, &chunkv1alpha1.CreateFlavorVersionRequest{
		Version: chunk.FlavorVersionToTransport(flavorVersion),
	})
	require.NoError(t, err)

	transport := make([]*chunkv1alpha1.File, 0, len(files))
	for _, file := range files {
		transport = append(transport, &chunkv1alpha1.File{
			Path: file.Path,
			Data: file.Data,
		})
	}

	_, err = client.SaveFlavorFiles(ctx, &chunkv1alpha1.SaveFlavorFilesRequest{
		FlavorVersionId: resp.GetVersion().Id,
		Files:           transport,
	})
	require.NoError(t, err)

	_, err = client.SaveFlavorFiles(ctx, &chunkv1alpha1.SaveFlavorFilesRequest{
		FlavorVersionId: resp.GetVersion().Id,
		Files:           transport,
	})
	require.ErrorIs(t, err, chunk.ErrFilesAlreadyExist)
}
