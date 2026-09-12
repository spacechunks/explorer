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
	"log/slog"
	"os"
	"testing"

	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/contextkey"
	"github.com/spacechunks/explorer/internal/file"
	"github.com/spacechunks/explorer/internal/mock"
	"github.com/spacechunks/explorer/internal/resource"
	"github.com/spacechunks/explorer/test/fixture"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGetFilesToUpload(t *testing.T) {
	tests := []struct {
		name     string
		version  resource.FlavorVersion
		existing map[string]struct{}
		expected []file.Hash
	}{
		{
			name:     "nothing in blob store yet",
			version:  fixture.FlavorVersion(),
			existing: map[string]struct{}{},
			expected: fixture.FlavorVersion().FileHashes,
		},
		{
			name:    "only files missing in blob store are returned",
			version: fixture.FlavorVersion(),
			existing: map[string]struct{}{
				"pppppppppppppppp": {},
				"cooooooooooooooo": {},
			},
			expected: []file.Hash{
				{
					Path: "server.properties",
					Hash: "server-prop-hash",
				},
			},
		},
		{
			// versions from before the blob index might carry the flag
			// even though their files never made it to the blob store.
			name: "files_uploaded flag is not trusted",
			version: fixture.FlavorVersion(func(v *resource.FlavorVersion) {
				v.FilesUploaded = true
			}),
			existing: map[string]struct{}{
				"pppppppppppppppp": {},
			},
			expected: []file.Hash{
				{
					Path: "plugins/myplugin/config.json",
					Hash: "cooooooooooooooo",
				},
				{
					Path: "server.properties",
					Hash: "server-prop-hash",
				},
			},
		},
		{
			name:    "everything present",
			version: fixture.FlavorVersion(),
			existing: map[string]struct{}{
				"pppppppppppppppp": {},
				"cooooooooooooooo": {},
				"server-prop-hash": {},
			},
			expected: []file.Hash{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx        = context.Background()
				mockRepo   = mock.NewMockChunkRepository(t)
				mockAccess = mock.NewMockAuthzAccessEvaluator(t)
				versionID  = "019da178-1347-7259-9f12-867550ae078d"
			)

			svc, err := chunk.NewService(
				slog.New(slog.NewTextHandler(os.Stdout, nil)),
				mockRepo,
				nil,
				nil,
				mockAccess,
				chunk.Config{},
				mock.NewMockUserRepository(t),
			)
			require.NoError(t, err)

			ctx = context.WithValue(ctx, contextkey.ActorIDPID, "blabla")

			mockAccess.EXPECT().
				AccessAuthorized(mocky.Anything, mocky.AnythingOfType("authz.AccessRuleOption")).
				Return(nil)

			mockRepo.EXPECT().
				FlavorIDByFlavorVersionID(mocky.Anything, versionID).
				Return(fixture.Flavor().ID, nil)

			mockRepo.EXPECT().
				FlavorByID(mocky.Anything, fixture.Flavor().ID).
				Return(fixture.Flavor(), nil)

			mockRepo.EXPECT().
				FlavorVersionByID(mocky.Anything, versionID).
				Return(tt.version, nil)

			mockRepo.EXPECT().
				ExistingBlobHashes(mocky.Anything, mocky.Anything).
				Return(tt.existing, nil)

			actual, err := svc.GetFilesToUpload(ctx, versionID)
			require.NoError(t, err)
			require.Equal(t, tt.expected, actual)
		})
	}
}
