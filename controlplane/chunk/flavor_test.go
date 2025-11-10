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

package chunk_test

import (
	"context"
	"testing"

	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/contextkey"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/resource"
	"github.com/spacechunks/explorer/internal/file"
	"github.com/spacechunks/explorer/internal/mock"
	"github.com/spacechunks/explorer/test/fixture"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCreateFlavor(t *testing.T) {
	chunkID := "somethingsomething"
	tests := []struct {
		name     string
		flavor   resource.Flavor
		expected resource.Flavor
		err      error
		prep     func(*mock.MockChunkRepository, *mock.MockAuthzAccessEvaluator)
	}{
		{
			name:     "works",
			flavor:   fixture.Flavor(),
			expected: fixture.Flavor(),
			prep: func(repo *mock.MockChunkRepository, access *mock.MockAuthzAccessEvaluator) {
				access.EXPECT().
					AccessAuthorized(
						mocky.Anything,
						// mockery doesn't like it if we put in the real option:
						// panic: cannot use Func in expectations. Use mock.AnythingOfType("authz.AccessRuleOption") [recovered, repanicked]
						mocky.AnythingOfType("authz.AccessRuleOption"),
					).
					Return(nil)

				repo.EXPECT().
					FlavorNameExists(mocky.Anything, chunkID, fixture.Flavor().Name).
					Return(false, nil)
				repo.EXPECT().
					CreateFlavor(mocky.Anything, chunkID, fixture.Flavor()).
					Return(fixture.Flavor(), nil)
			},
		},
		{
			name:   "flavor name already exists",
			err:    apierrs.ErrFlavorNameExists,
			flavor: fixture.Flavor(),
			prep: func(repo *mock.MockChunkRepository, access *mock.MockAuthzAccessEvaluator) {
				access.EXPECT().
					AccessAuthorized(
						mocky.Anything,
						mocky.AnythingOfType("authz.AccessRuleOption"),
					).
					Return(nil)

				repo.EXPECT().
					FlavorNameExists(mocky.Anything, chunkID, fixture.Flavor().Name).
					Return(true, nil)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx        = context.Background()
				mockRepo   = mock.NewMockChunkRepository(t)
				mockAccess = mock.NewMockAuthzAccessEvaluator(t)
				svc        = chunk.NewService(mockRepo, nil, nil, mockAccess, chunk.Config{})
			)

			ctx = context.WithValue(ctx, contextkey.ActorID, "blabla")

			tt.prep(mockRepo, mockAccess)

			actual, err := svc.CreateFlavor(ctx, chunkID, tt.flavor)

			if tt.err != nil {
				require.ErrorAs(t, err, &tt.err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestCreateFlavorVersion(t *testing.T) {
	tests := []struct {
		name         string
		prevVersion  resource.FlavorVersion
		newVersion   resource.FlavorVersion
		expectedDiff resource.FlavorVersionDiff
		prep         func(
			*mock.MockChunkRepository,
			resource.FlavorVersion,
			resource.FlavorVersion,
			*mock.MockAuthzAccessEvaluator,
		)
		err error
	}{
		{
			name:        "works",
			prevVersion: fixture.FlavorVersion(),
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
						Hash: "hash changed",
					},
					{
						Path: "plugins/myplugin.jar", // added
						Hash: "hash1",
					},
				}
				v.ChangeHash = "68df46974f6dc5fe"
			}),
			expectedDiff: resource.FlavorVersionDiff{
				Added: []file.Hash{
					{
						Path: "plugins/myplugin.jar",
						Hash: "hash1",
					},
				},
				Removed: []file.Hash{
					{
						Path: "plugins/myplugin/config.json",
						Hash: "cooooooooooooooo",
					},
				},
				Changed: []file.Hash{
					{
						Path: "server.properties",
						Hash: "hash changed",
					},
				},
			},
			prep: func(
				repo *mock.MockChunkRepository,
				newVersion resource.FlavorVersion,
				prevVersion resource.FlavorVersion,
				access *mock.MockAuthzAccessEvaluator,
			) {
				access.EXPECT().
					AccessAuthorized(
						mocky.Anything,
						mocky.AnythingOfType("authz.AccessRuleOption"),
					).
					Return(nil)

				repo.EXPECT().
					FlavorVersionExists(mocky.Anything, fixture.Flavor().ID, newVersion.Version).
					Return(false, nil)

				repo.EXPECT().
					MinecraftVersionExists(mocky.Anything, newVersion.MinecraftVersion).
					Return(true, nil)

				repo.EXPECT().
					LatestFlavorVersion(mocky.Anything, fixture.Flavor().ID).
					Return(prevVersion, nil)

				repo.EXPECT().
					CreateFlavorVersion(mocky.Anything, fixture.FlavorID, newVersion, prevVersion.ID).
					Return(newVersion, nil)
			},
		},
		{
			name:        "version hash mismatch",
			prevVersion: fixture.FlavorVersion(),
			newVersion: fixture.FlavorVersion(func(v *resource.FlavorVersion) {
				v.Hash = "some-not-matching-hash"
				v.FileHashes = []file.Hash{
					// plugins/myplugin/config.json not present -> its removed
					{
						Path: "paper.yml", // unchanged
						Hash: "paper.yml-hash",
					},
					{
						Path: "server.properties", // changed
						Hash: "hash changed",
					},
					{
						Path: "plugins/myplugin.jar", // added
						Hash: "hash1",
					},
				}
			}),
			prep: func(
				repo *mock.MockChunkRepository,
				newVersion resource.FlavorVersion,
				prevVersion resource.FlavorVersion,
				access *mock.MockAuthzAccessEvaluator,
			) {
				access.EXPECT().
					AccessAuthorized(
						mocky.Anything,
						mocky.AnythingOfType("authz.AccessRuleOption"),
					).
					Return(nil)

				repo.EXPECT().
					FlavorVersionExists(mocky.Anything, fixture.Flavor().ID, newVersion.Version).
					Return(false, nil)

				repo.EXPECT().
					MinecraftVersionExists(mocky.Anything, newVersion.MinecraftVersion).
					Return(true, nil)

				repo.EXPECT().
					LatestFlavorVersion(mocky.Anything, fixture.Flavor().ID).
					Return(prevVersion, nil)
			},
			err: apierrs.ErrHashMismatch,
		},
		{
			name:        "version already exists",
			prevVersion: fixture.FlavorVersion(),
			newVersion:  fixture.FlavorVersion(),
			prep: func(
				repo *mock.MockChunkRepository,
				newVersion resource.FlavorVersion,
				prevVersion resource.FlavorVersion,
				access *mock.MockAuthzAccessEvaluator,
			) {
				access.EXPECT().
					AccessAuthorized(
						mocky.Anything,
						mocky.AnythingOfType("authz.AccessRuleOption"),
					).
					Return(nil)

				repo.EXPECT().
					FlavorVersionExists(mocky.Anything, fixture.Flavor().ID, newVersion.Version).
					Return(true, nil)
			},
			err: apierrs.ErrFlavorVersionExists,
		},
		{
			name:        "minecraft version unsupported",
			prevVersion: fixture.FlavorVersion(),
			newVersion:  fixture.FlavorVersion(),
			prep: func(
				repo *mock.MockChunkRepository,
				newVersion resource.FlavorVersion,
				prevVersion resource.FlavorVersion,
				access *mock.MockAuthzAccessEvaluator,
			) {
				access.EXPECT().
					AccessAuthorized(
						mocky.Anything,
						mocky.AnythingOfType("authz.AccessRuleOption"),
					).
					Return(nil)

				repo.EXPECT().
					FlavorVersionExists(mocky.Anything, fixture.Flavor().ID, newVersion.Version).
					Return(false, nil)

				repo.EXPECT().
					MinecraftVersionExists(mocky.Anything, newVersion.MinecraftVersion).
					Return(false, nil)
			},
			err: apierrs.ErrMinecraftVersionNotSupported,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx        = context.Background()
				mockAccess = mock.NewMockAuthzAccessEvaluator(t)
				mockRepo   = mock.NewMockChunkRepository(t)
				svc        = chunk.NewService(mockRepo, nil, nil, mockAccess, chunk.Config{})
			)

			ctx = context.WithValue(ctx, contextkey.ActorID, "blabla")

			tt.prep(mockRepo, tt.newVersion, tt.prevVersion, mockAccess)

			actualNewVersion, actualDiff, err := svc.CreateFlavorVersion(ctx, fixture.FlavorID, tt.newVersion)

			if tt.err != nil {
				require.ErrorAs(t, err, &tt.err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.newVersion, actualNewVersion)
			require.Equal(t, tt.expectedDiff, actualDiff)
		})
	}
}
