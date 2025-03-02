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

package fixture

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/spacechunks/explorer/controlplane"
	"github.com/spacechunks/explorer/test"
	"github.com/stretchr/testify/require"
)

const ControlPlaneAddr = "localhost:9012"

func RunControlPlane(t *testing.T, pg *Postgres) {
	ctx, cancel := context.WithCancel(context.Background())
	pg.Run(t, ctx)

	var (
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
		server = controlplane.NewServer(logger, controlplane.Config{
			ListenAddr:   ControlPlaneAddr,
			DBConnString: pg.ConnString,
		})
	)

	t.Cleanup(func() {
		cancel()
	})

	go func() {
		require.NoError(t, server.Run(ctx))
	}()

	test.WaitServerReady(t, "tcp", ControlPlaneAddr, 20*time.Second)
}
