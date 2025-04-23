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
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/internal/mock"
	"github.com/spacechunks/explorer/test/fixture"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCreateFlavor(t *testing.T) {
	chunkID := "somethingsomething"
	tests := []struct {
		name     string
		flavor   chunk.Flavor
		expected chunk.Flavor
		err      error
		prep     func(*mock.MockChunkRepository)
	}{
		{
			name:     "works",
			flavor:   fixture.Flavor(),
			expected: fixture.Flavor(),
			prep: func(repo *mock.MockChunkRepository) {
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
			prep: func(repo *mock.MockChunkRepository) {
				repo.EXPECT().
					FlavorNameExists(mocky.Anything, chunkID, fixture.Flavor().Name).
					Return(true, nil)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx      = context.Background()
				mockRepo = mock.NewMockChunkRepository(t)
				svc      = chunk.NewService(mockRepo, nil)
			)

			tt.prep(mockRepo)

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

func TestListFlavors(t *testing.T) {
	chunkID := "somethingsomething"
	tests := []struct {
		name     string
		expected []chunk.Flavor
		err      error
		prep     func(*mock.MockChunkRepository, []chunk.Flavor)
	}{
		{
			name: "works",
			expected: []chunk.Flavor{
				fixture.Flavor(func(f *chunk.Flavor) {
					f.Name = "f1"
				}),
				fixture.Flavor(func(f *chunk.Flavor) {
					f.Name = "f2"
				}),
			},
			prep: func(repo *mock.MockChunkRepository, flavors []chunk.Flavor) {
				repo.EXPECT().
					ChunkExists(mocky.Anything, chunkID).
					Return(true, nil)
				repo.EXPECT().
					ListFlavorsByChunkID(mocky.Anything, chunkID).
					Return(flavors, nil)
			},
		},
		{
			name: "chunk does not exists",
			err:  apierrs.ErrInvalidChunkID,
			prep: func(repo *mock.MockChunkRepository, _ []chunk.Flavor) {
				repo.EXPECT().
					ChunkExists(mocky.Anything, chunkID).
					Return(false, nil)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx      = context.Background()
				mockRepo = mock.NewMockChunkRepository(t)
				svc      = chunk.NewService(mockRepo, nil)
			)

			tt.prep(mockRepo, tt.expected)

			flavors, err := svc.ListFlavors(ctx, chunkID)

			if tt.err != nil {
				require.ErrorIs(t, err, apierrs.ErrChunkNotFound)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, flavors)
		})
	}
}

func TestCreateFlavorVersion(t *testing.T) {
	tests := []struct {
		name         string
		prevVersion  chunk.FlavorVersion
		newVersion   chunk.FlavorVersion
		expectedDiff chunk.FlavorVersionDiff
		prep         func(*mock.MockChunkRepository, chunk.FlavorVersion, chunk.FlavorVersion)
		err          error
	}{
		{
			name:        "works",
			prevVersion: fixture.FlavorVersion(t),
			newVersion: fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
				v.Version = "v2"
				v.FileHashes = []chunk.FileHash{
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
			expectedDiff: chunk.FlavorVersionDiff{
				Added: []chunk.FileHash{
					{
						Path: "plugins/myplugin.jar",
						Hash: "hash1",
					},
				},
				Removed: []chunk.FileHash{
					{
						Path: "plugins/myplugin/config.json",
						Hash: "cooooooooooooooo",
					},
				},
				Changed: []chunk.FileHash{
					{
						Path: "server.properties",
						Hash: "hash changed",
					},
				},
			},
			prep: func(
				repo *mock.MockChunkRepository,
				newVersion chunk.FlavorVersion,
				prevVersion chunk.FlavorVersion,
			) {
				repo.EXPECT().
					FlavorVersionExists(mocky.Anything, fixture.Flavor().ID, newVersion.Version).
					Return(false, nil)

				repo.EXPECT().
					FlavorVersionByHash(mocky.Anything, newVersion.Hash).
					Return("", nil)

				repo.EXPECT().
					LatestFlavorVersion(mocky.Anything, fixture.Flavor().ID).
					Return(prevVersion, nil)

				repo.EXPECT().
					CreateFlavorVersion(mocky.Anything, newVersion, prevVersion.ID).
					Return(newVersion, nil)
			},
		},
		{
			name:        "version hash mismatch",
			prevVersion: fixture.FlavorVersion(t),
			newVersion: fixture.FlavorVersion(t, func(v *chunk.FlavorVersion) {
				v.Hash = "some-not-matching-hash"
				v.FileHashes = []chunk.FileHash{
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
				newVersion chunk.FlavorVersion,
				prevVersion chunk.FlavorVersion,
			) {
				repo.EXPECT().
					FlavorVersionExists(mocky.Anything, fixture.Flavor().ID, newVersion.Version).
					Return(false, nil)

				repo.EXPECT().
					FlavorVersionByHash(mocky.Anything, newVersion.Hash).
					Return("", nil)

				repo.EXPECT().
					LatestFlavorVersion(mocky.Anything, fixture.Flavor().ID).
					Return(prevVersion, nil)
			},
			err: apierrs.ErrHashMismatch,
		},
		{
			name:        "version already exists",
			prevVersion: fixture.FlavorVersion(t),
			newVersion:  fixture.FlavorVersion(t),
			prep: func(
				repo *mock.MockChunkRepository,
				newVersion chunk.FlavorVersion,
				prevVersion chunk.FlavorVersion,
			) {
				repo.EXPECT().
					FlavorVersionExists(mocky.Anything, fixture.Flavor().ID, newVersion.Version).
					Return(true, nil)
			},
			err: apierrs.ErrFlavorVersionExists,
		},
		{
			name:        "version is duplicate",
			prevVersion: fixture.FlavorVersion(t),
			newVersion:  fixture.FlavorVersion(t),
			prep: func(
				repo *mock.MockChunkRepository,
				newVersion chunk.FlavorVersion,
				prevVersion chunk.FlavorVersion,
			) {
				repo.EXPECT().
					FlavorVersionExists(mocky.Anything, fixture.Flavor().ID, newVersion.Version).
					Return(false, nil)

				repo.EXPECT().
					FlavorVersionByHash(mocky.Anything, newVersion.Hash).
					Return(newVersion.Hash, nil)
			},
			err: apierrs.FlavorVersionDuplicate(fixture.FlavorVersion(t).Version),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx      = context.Background()
				mockRepo = mock.NewMockChunkRepository(t)
				svc      = chunk.NewService(mockRepo, nil)
			)

			tt.prep(mockRepo, tt.newVersion, tt.prevVersion)

			actualNewVersion, actualDiff, err := svc.CreateFlavorVersion(ctx, tt.newVersion)

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
