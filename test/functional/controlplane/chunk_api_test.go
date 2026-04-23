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
	"github.com/jackc/pgx/v5"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/chunk"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/resource"
	"github.com/spacechunks/explorer/internal/file"
	"github.com/spacechunks/explorer/internal/image"
	imgtestdata "github.com/spacechunks/explorer/internal/image/testdata"
	"github.com/spacechunks/explorer/internal/ptr"
	"github.com/spacechunks/explorer/test"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/spacechunks/explorer/test/functional/controlplane/testdata"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/xxh3"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestAPICreateChunk(t *testing.T) {
	tests := []struct {
		name     string
		expected resource.Chunk
		err      error
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
			err: apierrs.ErrNameTooLong.GRPCStatus().Err(),
		},
		{
			name: "description too long",
			expected: fixture.Chunk(func(c *resource.Chunk) {
				c.Description = strings.Repeat("a", resource.MaxChunkDescriptionChars+1)
			}),
			err: apierrs.ErrDescriptionTooLong.GRPCStatus().Err(),
		},
		{
			name: "too many tags",
			expected: fixture.Chunk(func(c *resource.Chunk) {
				c.Tags = slices.Repeat([]string{"a"}, resource.MaxChunkTags+1)
			}),
			err: apierrs.ErrTooManyTags.GRPCStatus().Err(),
		},
		{
			name: "invalid name",
			expected: fixture.Chunk(func(c *resource.Chunk) {
				c.Name = ""
			}),
			err: apierrs.ErrInvalidName.GRPCStatus().Err(),
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

			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}

			require.NoError(t, err)

			tt.expected.Owner = u

			if d := cmp.Diff(
				chunk.ChunkToTransport(tt.expected),
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

	chunks := []resource.Chunk{
		fixture.Chunk(func(c *resource.Chunk) {
			c.ID = test.NewUUIDv7(t)
			c.Owner = u
			c.Flavors = []resource.Flavor{
				fixture.Flavor(func(f *resource.Flavor) {
					f.ID = test.NewUUIDv7(t)
					f.Name = "ddddawq31423452"
				}),
			}
		}),
		fixture.Chunk(func(c *resource.Chunk) {
			c.ID = test.NewUUIDv7(t)
			c.Owner = u
			c.Flavors = []resource.Flavor{
				fixture.Flavor(func(f *resource.Flavor) {
					f.ID = test.NewUUIDv7(t)
					f.Name = "dawdawdawd"
				}),
			}
		}),
		fixture.Chunk(func(c *resource.Chunk) { // this one should not appear
			c.ID = test.NewUUIDv7(t)
			c.Owner = u
			c.DeletedAt = new(time.Now())
		}),
	}

	for i := range chunks {
		cp.Postgres.CreateChunk(t, &chunks[i], fixture.CreateOptionsAll)
	}

	cp.AddUserAPIKey(t, &ctx, chunks[0].Owner)

	resp, err := client.ListChunks(ctx, &chunkv1alpha1.ListChunksRequest{})
	require.NoError(t, err)

	expected := make([]*chunkv1alpha1.Chunk, 0)
	for _, c := range chunks {
		if c.DeletedAt != nil {
			continue
		}
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
		test.IgnoredProtoUserFields,
	); d != "" {
		t.Fatalf("diff (-want +got):\n%s", d)
	}
}

func TestUpdateChunk(t *testing.T) {
	tests := []struct {
		name string
		c    *resource.Chunk
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
				Name: strings.Repeat("a", resource.MaxChunkNameChars+1),
			},
			err: apierrs.ErrNameTooLong.GRPCStatus().Err(),
		},
		{
			name: "description too long",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Description: strings.Repeat("a", resource.MaxChunkDescriptionChars+1),
			},
			err: apierrs.ErrDescriptionTooLong.GRPCStatus().Err(),
		},
		{
			name: "too many tags",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Tags: slices.Repeat([]string{"a"}, resource.MaxChunkTags+1),
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
		{
			name: "chunk not found because it's deleted",
			req: &chunkv1alpha1.UpdateChunkRequest{
				Id:   fixture.Chunk().ID,
				Name: "new-name",
			},
			c: new(fixture.Chunk(func(tmp *resource.Chunk) {
				tmp.DeletedAt = new(time.Time)
			})),
			err: apierrs.ErrChunkNotFound.GRPCStatus().Err(),
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

			if tt.err != nil {
				require.ErrorAs(t, err, &tt.err)
				return
			}

			require.NoError(t, err)

			expected := chunk.ChunkToTransport(*tt.c)

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
		name       string
		flavorName string
		chunkID    string
		err        error
	}{
		{
			name:       "works",
			flavorName: fixture.Flavor().Name,
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
	c := fixture.Chunk()

	tests := []struct {
		name        string
		prevVersion *resource.FlavorVersion
		newVersion  resource.FlavorVersion
		diff        resource.FlavorVersionDiff
		err         error
	}{
		{
			name:       "create initial version",
			newVersion: fixture.FlavorVersion(),
			diff: resource.FlavorVersionDiff{
				Added: fixture.FlavorVersion().FileHashes,
			},
		},
		{
			name:        "create second version with changed files",
			prevVersion: ptr.Pointer(fixture.FlavorVersion()),
			newVersion: fixture.FlavorVersion(func(v *resource.FlavorVersion) {
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
			diff: resource.FlavorVersionDiff{
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
			prevVersion: ptr.Pointer(fixture.FlavorVersion()),
			newVersion:  fixture.FlavorVersion(),
			err:         apierrs.ErrFlavorVersionExists.GRPCStatus().Err(),
		},
		{
			name:        "version hash mismatch",
			prevVersion: ptr.Pointer(fixture.FlavorVersion()),
			newVersion: fixture.FlavorVersion(func(v *resource.FlavorVersion) {
				v.Version = "v2"
				v.Hash = "wrong-hash"
			}),
			err: apierrs.ErrHashMismatch.GRPCStatus().Err(),
		},
		{
			name:        "unsupported minecraft version",
			prevVersion: ptr.Pointer(fixture.FlavorVersion()),
			newVersion: fixture.FlavorVersion(func(v *resource.FlavorVersion) {
				v.Version = "v2"
				v.MinecraftVersion = "abcdef"
			}),
			err: apierrs.ErrMinecraftVersionNotSupported.GRPCStatus().Err(),
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
					FlavorId: c.Flavors[0].ID,
					Version:  chunk.FlavorVersionToTransport(*tt.prevVersion),
				})
				require.NoError(t, err)
			}

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

			if tt.err == nil {
				fakes3.UploadObject(t, blob.ChangeSetKey(flavorVersionID), testdata.FullChangeSetFile)
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
		{
			name: "changeset file too large",
			req: &chunkv1alpha1.GetUploadURLRequest{
				FlavorVersionId:  "blabla",
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
				q := `UPDATE flavor_versions SET files_uploaded = true WHERE id = $1`
				_, err := cp.Postgres.Pool.Exec(ctx, q, c.Flavors[0].Versions[0].ID)
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
		otherUser = fixture.User(func(tmp *resource.User) {
			tmp.Nickname = "other-nickname"
			tmp.Email = "other-email@example.com"
		})
	)

	cp.Run(t)
	cp.Postgres.CreateChunk(t, &c, fixture.CreateOptionsAll)
	cp.Postgres.CreateUser(t, &otherUser)

	client := cp.ChunkClient(t)
	cp.AddUserAPIKey(t, &ctx, otherUser)

	_, err := client.CreateFlavorVersion(ctx, &chunkv1alpha1.CreateFlavorVersionRequest{
		FlavorId: c.Flavors[0].ID,
		Version:  &chunkv1alpha1.FlavorVersion{},
	})

	require.ErrorIs(t, err, apierrs.ErrPermissionDenied.GRPCStatus().Err())
}

func TestUserThatIsNotOwnerCannotUpdateChunk(t *testing.T) {
	var (
		ctx       = context.Background()
		cp        = fixture.NewControlPlane(t)
		c         = fixture.Chunk()
		otherUser = fixture.User(func(tmp *resource.User) {
			tmp.Nickname = "other-nickname"
			tmp.Email = "other-email@example.com"
		})
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
		otherUser = fixture.User(func(tmp *resource.User) {
			tmp.Nickname = "other-nickname"
			tmp.Email = "other-email@example.com"
		})
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
		otherUser = fixture.User(func(tmp *resource.User) {
			tmp.Nickname = "other-nickname"
			tmp.Email = "other-email@example.com"
		})
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
		otherUser = fixture.User(func(tmp *resource.User) {
			tmp.Nickname = "other-nickname"
			tmp.Email = "other-email@example.com"
		})
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
		c.Flavors[0],
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
		chunk.ChunkToTransport(expected),
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
		other = fixture.User(func(tmp *resource.User) {
			tmp.Email = "other@example.com"
			tmp.Nickname = "other"
		})
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
					FlavorId: flavor.ID,
					Version: &chunkv1alpha1.FlavorVersion{
						Version:          "v1",
						Hash:             "awdawdawdawd",
						MinecraftVersion: fixture.MinecraftVersion,
					},
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
					ChunkId:         chunk.ID,
					FlavorVersionId: flavor.Versions[0].ID,
				})
				return err
			},
			err: apierrs.ErrFlavorVersionNotFound.GRPCStatus().Err(),
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
			FlavorId: f.ID,
			Version:  chunk.FlavorVersionToTransport(f.Versions[0]),
		})
		require.ErrorIsf(t, err, apierrs.ErrNotFound.GRPCStatus().Err(), "create flavor version (%s)", f.Name)

		_, err = insClient.RunFlavorVersion(ctx, &instancev1alpha1.RunFlavorVersionRequest{
			ChunkId:         c.ID,
			FlavorVersionId: f.Versions[0].ID,
		})
		require.ErrorIsf(t, err, apierrs.ErrChunkNotFound.GRPCStatus().Err(), "run flavor version (%s)", f.Name)
	}
}

func TestOnlyOwnerCanDeleteChunk(t *testing.T) {
	var (
		ctx   = context.Background()
		cp    = fixture.NewControlPlane(t)
		c     = fixture.Chunk()
		u     = fixture.User()
		other = fixture.User(func(tmp *resource.User) {
			tmp.Email = "other@example.com"
			tmp.Nickname = "other"
		})
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
