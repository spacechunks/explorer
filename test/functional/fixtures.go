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
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	proxyv1alpha1 "github.com/spacechunks/platform/api/platformd/proxy/v1alpha1"
	"github.com/spacechunks/platform/internal/platformd/proxy"
	"github.com/spacechunks/platform/internal/platformd/proxy/xds"
	"github.com/spacechunks/platform/test"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	// grpc client does not accept @ as abstract socket identifier,
	// so do not include it in the address string.
	proxyAddr = "/run/platformd/platformd.sock"

	envoyAdminAddr = "127.0.0.1:5555"
	dnsUpstream    = netip.MustParseAddrPort("127.0.0.1:53")
)

func runProxyFixture(ctx context.Context, t *testing.T) {
	var (
		logger   = slog.New(slog.NewTextHandler(os.Stdout, nil))
		grpcServ = grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
		ca       = cache.NewSnapshotCache(true, cache.IDHash{}, nil)
		svc      = proxy.NewService(
			logger,
			proxy.Config{
				DNSUpstream: dnsUpstream,
			}, xds.NewMap("proxy-0", ca),
		)
		proxyServ  = proxy.NewServer(svc)
		envoyImage = os.Getenv("FUNCTESTS_ENVOY_IMAGE")
		envoyCfg   = os.Getenv("FUNCTESTS_ENVOY_CONFIG")
	)

	path, err := filepath.Abs(envoyCfg)
	require.NoError(t, err)

	req := testcontainers.ContainerRequest{
		Image: envoyImage,
		Cmd: []string{
			"-c", "/etc/envoy/config.yaml",
		},
		HostConfigModifier: func(cfg *container.HostConfig) {
			cfg.NetworkMode = "host"
			cfg.AutoRemove = true
			cfg.Mounts = []mount.Mount{
				{
					Type:   mount.TypeBind,
					Source: path,
					Target: "/etc/envoy/config.yaml",
				},
			}
		},
	}

	_, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	test.WaitServerReady(t, "tcp", envoyAdminAddr, 20*time.Second)

	proxyv1alpha1.RegisterProxyServiceServer(grpcServ, proxyServ)
	xds.CreateAndRegisterServer(context.Background(), logger, grpcServ, ca)

	require.NoError(t, svc.ApplyGlobalResources(ctx))

	unixSock, err := net.Listen("unix", "@"+proxyAddr)
	require.NoError(t, err)
	t.Cleanup(func() {
		grpcServ.Stop()
		unixSock.Close()
	})
	go func() {
		require.NoError(t, grpcServ.Serve(unixSock))
	}()

	test.WaitServerReady(t, "unix", "@"+proxyAddr, 20*time.Second)
}
