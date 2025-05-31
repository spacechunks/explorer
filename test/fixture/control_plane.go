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
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/spacechunks/explorer/controlplane"
	"github.com/spacechunks/explorer/test"
	"github.com/stretchr/testify/require"
)

const (
	ControlPlaneAddr = "localhost:9012"
	BaseImage        = "base-image:latest"
)

type ControlPlaneRunOption func(*ControlPlaneRunOptions)

type ControlPlaneRunOptions struct {
	OCIRegistryEndpoint string
}

func WithOCIRegistryEndpoint(endpoint string) ControlPlaneRunOption {
	return func(opts *ControlPlaneRunOptions) {
		opts.OCIRegistryEndpoint = endpoint
	}
}

func RunControlPlane(t *testing.T, pg *Postgres, opts ...ControlPlaneRunOption) {
	ctx := context.Background()
	pg.Run(t, ctx)

	defaultOpts := ControlPlaneRunOptions{
		OCIRegistryEndpoint: "http://localhost:5000",
	}

	for _, opt := range opts {
		opt(&defaultOpts)
	}

	var (
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
		server = controlplane.NewServer(logger, controlplane.Config{
			ListenAddr:         ControlPlaneAddr,
			DBConnString:       pg.ConnString,
			MaxGRPCMessageSize: 1_000_000_000,
			OCIRegistry:        defaultOpts.OCIRegistryEndpoint,
			OCIRegistryUser:    OCIRegsitryUser,
			OCIRegistryPass:    OCIRegistryPass,
			BaseImage:          fmt.Sprintf("%s/base-image:latest", defaultOpts.OCIRegistryEndpoint),
			ImageCacheDir:      t.TempDir(),
		})
	)

	t.Cleanup(func() {
		server.Stop()
	})

	go func() {
		require.NoError(t, server.Run(ctx))
	}()

	test.WaitServerReady(t, "tcp", ControlPlaneAddr, 20*time.Second)
}
