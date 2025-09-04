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
	"errors"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	"github.com/spacechunks/explorer/controlplane/chunk"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/internal/file"
	"github.com/spacechunks/explorer/internal/image"
	imgtestdata "github.com/spacechunks/explorer/internal/image/testdata"
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
			err: apierrs.ErrNameTooLong.GRPCStatus().Err(),
		},
		{
			name: "description too long",
			expected: fixture.Chunk(func(c *chunk.Chunk) {
				c.Description = strings.Repeat("a", chunk.MaxChunkDescriptionChars+1)
			}),
			err: apierrs.ErrDescriptionTooLong.GRPCStatus().Err(),
		},
		{
			name: "too many tags",
			expected: fixture.Chunk(func(c *chunk.Chunk) {
				c.Tags = slices.Repeat([]string{"a"}, chunk.MaxChunkTags+1)
			}),
			err: apierrs.ErrTooManyTags.GRPCStatus().Err(),
		},
		{
			name: "invalid name",
			expected: fixture.Chunk(func(c *chunk.Chunk) {
				c.Name = ""
			}),
			err: apierrs.ErrInvalidName.GRPCStatus().Err(),
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
		name    string
		chunkID string
		err     error
	}{
		{
			name: "works",
		},
		{
			name:    "not found",
			chunkID: test.NewUUIDv7(t),
			err:     apierrs.ErrChunkNotFound.GRPCStatus().Err(),
		},
		{
			name:    "invalid id",
			chunkID: "invalid",
			err:     apierrs.ErrInvalidChunkID.GRPCStatus().Err(),
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

			if tt.chunkID == "" {
				pg.CreateChunk(t, &c, fixture.CreateOptionsAll)
				tt.chunkID = c.ID
			}

			conn, err := grpc.NewClient(
				fixture.ControlPlaneAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			client := chunkv1alpha1.NewChunkServiceClient(conn)

			resp, err := client.GetChunk(ctx, &chunkv1alpha1.GetChunkRequest{
				Id: tt.chunkID,
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
				test.IgnoredProtoFlavorVersionFields,
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
		pg.CreateChunk(t, &chunks[i], fixture.CreateOptionsAll)
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

	sort.Slice(expected, func(i, j int) bool {
		return strings.Compare(expected[i].GetId(), expected[j].GetId()) < 0
	})

	sort.Slice(resp.GetChunks(), func(i, j int) bool {
		return strings.Compare(resp.GetChunks()[i].GetId(), resp.GetChunks()[j].GetId()) < 0
	})

	if d := cmp.Diff(
		expected,
		resp.GetChunks(),
		protocmp.Transform(),
		test.IgnoredProtoChunkFields,
		test.IgnoredProtoFlavorFields,
		test.IgnoredProtoFlavorVersionFields,
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
			err: apierrs.ErrChunkNotFound.GRPCStatus().Err(),
		},
		{
			name: "name too long",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Name: strings.Repeat("a", chunk.MaxChunkNameChars+1),
			},
			err: apierrs.ErrNameTooLong.GRPCStatus().Err(),
		},
		{
			name: "description too long",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Description: strings.Repeat("a", chunk.MaxChunkDescriptionChars+1),
			},
			err: apierrs.ErrDescriptionTooLong.GRPCStatus().Err(),
		},
		{
			name: "too many tags",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Tags: slices.Repeat([]string{"a"}, chunk.MaxChunkTags+1),
			},
			err: apierrs.ErrTooManyTags.GRPCStatus().Err(),
		},
		{
			name: "invalid chunk id",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Id: "invalid",
			},
			err: apierrs.ErrInvalidChunkID.GRPCStatus().Err(),
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

			pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

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
				expected,
				resp.GetChunk(),
				protocmp.Transform(),
				test.IgnoredProtoChunkFields,
				test.IgnoredProtoFlavorFields,
				test.IgnoredProtoFlavorVersionFields,
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
		chunkID    string
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
			err:        apierrs.ErrFlavorNameExists.GRPCStatus().Err(),
		},
		{
			name:       "invalid chunk id",
			flavorName: fixture.Flavor().Name,
			chunkID:    "invalid",
			err:        apierrs.ErrInvalidChunkID.GRPCStatus().Err(),
		},
		{
			name: "invalid flavor name",
			err:  apierrs.ErrInvalidName.GRPCStatus().Err(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				pg  = fixture.NewPostgres()
			)

			fixture.RunControlPlane(t, pg)

			pg.CreateChunk(t, &c, fixture.CreateOptions{})

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

			if tt.chunkID == "" {
				tt.chunkID = c.ID
			}

			resp, err := client.CreateFlavor(ctx, &chunkv1alpha1.CreateFlavorRequest{
				ChunkId: tt.chunkID,
				Name:    tt.flavorName,
			})

			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
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
	var (
		c = fixture.Chunk()
	)

	tests := []struct {
		name        string
		prevVersion *chunk.FlavorVersion
		newVersion  chunk.FlavorVersion
		diff        chunk.FlavorVersionDiff
		err         error
	}{
		{
			name:       "create initial version",
			newVersion: fixture.FlavorVersion(t),
			diff: chunk.FlavorVersionDiff{
				Added: fixture.FlavorVersion(t).FileHashes,
			},
		},
		{
			name:        "create second version with changed files",
			prevVersion: ptr.Pointer(fixture.FlavorVersion(t)),
			newVersion: fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
				v.Version = "v2"
				v.FileHashes = []file.Hash{
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
				Added: []file.Hash{
					{
						Path: "plugins/myplugin.jar",
						Hash: "yyyyyyyyyyyyyyyy",
					},
				},
				Changed: []file.Hash{
					{
						Path: "server.properties",
						Hash: "cccccccccccccccc",
					},
				},
				Removed: []file.Hash{
					{
						Path: "plugins/myplugin/config.json",
						Hash: "cooooooooooooooo",
					},
				},
			},
		},
		{
			name:        "version already exists",
			prevVersion: ptr.Pointer(fixture.FlavorVersion(t)),
			newVersion:  fixture.FlavorVersion(t),
			err:         apierrs.ErrFlavorVersionExists.GRPCStatus().Err(),
		},
		{
			name:        "version hash mismatch",
			prevVersion: ptr.Pointer(fixture.FlavorVersion(t)),
			newVersion: fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
				v.Version = "v2"
				v.Hash = "wrong-hash"
			}),
			err: apierrs.ErrHashMismatch.GRPCStatus().Err(),
		},
		{
			name:        "duplicate version",
			prevVersion: ptr.Pointer(fixture.FlavorVersion(t)),
			newVersion: fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
				v.Version = "v2"
			}),
			err: apierrs.FlavorVersionDuplicate("v1").GRPCStatus().Err(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				pg  = fixture.NewPostgres()
			)

			fixture.RunControlPlane(t, pg)

			pg.CreateChunk(t, &c, fixture.CreateOptions{
				WithFlavors: true,
			})
			pg.DB.CreateFlavor(ctx, c.ID, fixture.Flavor())

			conn, err := grpc.NewClient(
				fixture.ControlPlaneAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			client := chunkv1alpha1.NewChunkServiceClient(conn)

			if tt.prevVersion != nil {
				_, err := client.CreateFlavorVersion(ctx, &chunkv1alpha1.CreateFlavorVersionRequest{
					FlavorId: c.Flavors[0].ID,
					Version:  chunk.FlavorVersionToTransport(*tt.prevVersion),
				})
				require.NoError(t, err)
			}

			//tt.newVersion.FlavorID = c.Flavors[0].ID
			version := chunk.FlavorVersionToTransport(tt.newVersion)

			resp, err := client.CreateFlavorVersion(ctx, &chunkv1alpha1.CreateFlavorVersionRequest{
				FlavorId: c.Flavors[0].ID,
				Version:  version,
			})

			if err != nil {
				if tt.err != nil {
					require.ErrorIs(t, err, tt.err)
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
	files := []file.Object{
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
		files   []file.Object
		version chunk.FlavorVersion
		err     error
	}{
		{
			name:  "works",
			files: files,
			version: fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
				v.FileHashes = []file.Hash{
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
				v.FileHashes = []file.Hash{
					{
						Path: "plugins/testadata1",
						Hash: "1f47515caccc8b7c",
					},
				}
			}),
			err: apierrs.ErrHashMismatch.GRPCStatus().Err(),
		},
		{
			name:  "hash mismatch - unexpected file",
			files: files,
			version: fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
				v.FileHashes = []file.Hash{
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
			err: apierrs.ErrHashMismatch.GRPCStatus().Err(),
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
			pg.CreateChunk(t, &c, fixture.CreateOptions{
				WithFlavors: true,
			})

			conn, err := grpc.NewClient(
				fixture.ControlPlaneAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			client := chunkv1alpha1.NewChunkServiceClient(conn)

			resp, err := client.CreateFlavorVersion(ctx, &chunkv1alpha1.CreateFlavorVersionRequest{
				FlavorId: c.Flavors[0].ID,
				Version:  chunk.FlavorVersionToTransport(tt.version),
			})
			require.NoError(t, err)

			files := make([]*chunkv1alpha1.File, 0, len(tt.files))
			for _, f := range tt.files {
				files = append(files, &chunkv1alpha1.File{
					Path: f.Path,
					Data: f.Data,
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
		files = []file.Object{
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
			v.FileHashes = []file.Hash{
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
	pg.CreateChunk(t, &c, fixture.CreateOptions{
		WithFlavors: true,
	})

	conn, err := grpc.NewClient(
		fixture.ControlPlaneAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	client := chunkv1alpha1.NewChunkServiceClient(conn)

	resp, err := client.CreateFlavorVersion(ctx, &chunkv1alpha1.CreateFlavorVersionRequest{
		FlavorId: c.Flavors[0].ID,
		Version:  chunk.FlavorVersionToTransport(flavorVersion),
	})
	require.NoError(t, err)

	transport := make([]*chunkv1alpha1.File, 0, len(files))
	for _, f := range files {
		transport = append(transport, &chunkv1alpha1.File{
			Path: f.Path,
			Data: f.Data,
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
	require.ErrorIs(t, err, apierrs.ErrFilesAlreadyExist.GRPCStatus().Err())
}

func TestBuildFlavorVersion(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "works",
		},
		{
			name: "files not uploaded",
			err:  apierrs.ErrFlavorFilesNotUploaded.GRPCStatus().Err(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx      = context.Background()
				pg       = fixture.NewPostgres()
				endpoint = fixture.RunRegistry(t)
				c        = fixture.Chunk()
				auth     = remote.WithAuth(&image.Auth{
					Username: fixture.OCIRegsitryUser,
					Password: fixture.OCIRegistryPass,
				})
			)

			fixture.RunControlPlane(t, pg, fixture.WithOCIRegistryEndpoint(endpoint))
			fixture.RunFakeCRI(t)
			fixture.RunCheckpointAPIFixtures(t, fixture.OCIRegsitryUser, fixture.OCIRegistryPass)

			pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

			var (
				flavor  = c.Flavors[0]
				version = flavor.Versions[0]
			)

			pg.CreateBlobs(t, version)
			pg.InsertNode(t)

			// push base image needed for testing

			pusher, err := remote.NewPusher(auth)
			require.NoError(t, err)

			baseImgRef, err := name.ParseReference(fmt.Sprintf("%s/%s", endpoint, fixture.BaseImage))
			require.NoError(t, err)

			err = pusher.Push(ctx, baseImgRef, imgtestdata.Image(t))
			require.NoError(t, err)

			conn, err := grpc.NewClient(
				fixture.ControlPlaneAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			client := chunkv1alpha1.NewChunkServiceClient(conn)

			if !errors.Is(tt.err, apierrs.ErrFlavorFilesNotUploaded.GRPCStatus().Err()) {
				_, err := pg.Pool.Exec(ctx, "UPDATE flavor_versions SET files_uploaded = true WHERE id = $1", version.ID)
				require.NoError(t, err)
			}

			_, err = client.BuildFlavorVersion(ctx, &chunkv1alpha1.BuildFlavorVersionRequest{
				FlavorVersionId: version.ID,
			})

			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}

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

					if !slices.Contains(cat, version.ID) {
						continue
					}

					h, err := p.Lister(ctx, reg.Repo(version.ID))
					require.NoError(t, err)

					for h.HasNext() {
						tags, err := h.Next(ctx)
						require.NoError(t, err)

						if slices.Contains(tags.Tags, "checkpoint") {
							// FIXME: test that state is set to completed
							return
						}
					}
				}
			}
		})
	}
}

func TestGetUploadURLWorks(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
		c   = fixture.Chunk()
	)

	fixture.RunFakeS3(t)
	fixture.RunControlPlane(t, pg)

	pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

	conn, err := grpc.NewClient(
		fixture.ControlPlaneAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	client := chunkv1alpha1.NewChunkServiceClient(conn)

	resp, err := client.GetUploadURL(ctx, &chunkv1alpha1.GetUploadURLRequest{
		FlavorVersionId: c.Flavors[0].Versions[0].ID,
		TarballHash:     "blabla",
	})
	require.NoError(t, err)

	u, err := url.Parse(resp.Url)
	require.NoError(t, err)

	require.Equal(t, "blabla", u.Query().Get("X-Amz-Checksum-Sha256"))
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
				pg  = fixture.NewPostgres()
				c   = fixture.Chunk()
			)

			fixture.RunFakeS3(t)
			fixture.RunControlPlane(t, pg)

			pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

			conn, err := grpc.NewClient(
				fixture.ControlPlaneAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			client := chunkv1alpha1.NewChunkServiceClient(conn)

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
		name string
		req  *chunkv1alpha1.GetUploadURLRequest
		err  error
	}{
		{
			name: "invalid flavor version id",
			req: &chunkv1alpha1.GetUploadURLRequest{
				FlavorVersionId: "blabla",
				TarballHash:     "blabla",
			},
			err: apierrs.ErrInvalidChunkID.GRPCStatus().Err(),
		},
		{
			name: "invalid tarball hash",
			req: &chunkv1alpha1.GetUploadURLRequest{
				FlavorVersionId: test.NewUUIDv7(t),
				TarballHash:     "",
			},
			err: apierrs.ErrInvalidHash.GRPCStatus().Err(),
		},
		{
			name: "files not uploaded",
			req:  &chunkv1alpha1.GetUploadURLRequest{},
			err:  apierrs.ErrFlavorFilesUploaded.GRPCStatus().Err(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				pg  = fixture.NewPostgres()
				c   = fixture.Chunk()
			)

			fixture.RunFakeS3(t)
			fixture.RunControlPlane(t, pg)

			pg.CreateChunk(t, &c, fixture.CreateOptionsAll)

			if errors.Is(tt.err, apierrs.ErrFlavorFilesUploaded.GRPCStatus().Err()) {
				q := `UPDATE flavor_versions SET files_uploaded = true WHERE id = $1`
				_, err := pg.Pool.Exec(ctx, q, c.Flavors[0].Versions[0].ID)
				require.NoError(t, err)

				tt.req = &chunkv1alpha1.GetUploadURLRequest{
					FlavorVersionId: c.Flavors[0].Versions[0].ID,
					TarballHash:     "blabla",
				}
			}

			conn, err := grpc.NewClient(
				fixture.ControlPlaneAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			client := chunkv1alpha1.NewChunkServiceClient(conn)

			_, err = client.GetUploadURL(ctx, tt.req)

			require.ErrorIs(t, err, tt.err)
		})
	}
}
