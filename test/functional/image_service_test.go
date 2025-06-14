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
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/spacechunks/explorer/controlplane/image"
	imgtestdata "github.com/spacechunks/explorer/controlplane/image/testdata"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/stretchr/testify/require"
)

func TestImagePull(t *testing.T) {
	var (
		ctx      = context.Background()
		cacheDir = t.TempDir()
		service  = image.NewService(
			slog.New(slog.NewTextHandler(os.Stdout, nil)),
			fixture.OCIRegsitryUser,
			fixture.OCIRegistryPass,
			cacheDir,
		)
		endpoint = fixture.RunRegistry(t)
	)

	ref := strings.ReplaceAll(endpoint, "http://", "") + "/test:latest"

	err := service.Push(ctx, imgtestdata.Image(t), ref)
	require.NoError(t, err)

	_, err = service.Pull(ctx, ref)
	require.NoError(t, err)

	require.FileExists(t, cacheDir+"/"+strings.ReplaceAll(ref, "/", "_"))
}
