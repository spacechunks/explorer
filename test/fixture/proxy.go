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
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	proxyv1alpha1 "github.com/spacechunks/explorer/api/platformd/proxy/v1alpha1"
	"github.com/spacechunks/explorer/platformd/proxy"
	"github.com/spacechunks/explorer/platformd/proxy/xds"
	"github.com/spacechunks/explorer/test"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	EnvoyAdminAddr = "127.0.0.1:5555"
	DNSUpstream    = netip.MustParseAddrPort("127.0.0.1:53")
	envoyCfg       = `
node:
  cluster: system
  id: proxy-0

admin:
  profile_path: /tmp/envoy.prof
  address:
    socket_address:
      address: 127.0.0.1
      port_value: 5555

dynamic_resources:
  ads_config:
    api_type: GRPC
    grpc_services:
    - envoy_grpc:
        cluster_name: ads
  cds_config:
    ads: {}
  lds_config:
    ads: {}

static_resources:
  clusters:
  - name: ads
    load_assignment:
      cluster_name: ads
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              pipe:
                path: '/tmp/platformd.sock'
                mode: 0777
    # It is recommended to configure either HTTP/2 or TCP keepalives in order to detect
    # connection issues, and allow Envoy to reconnect. TCP keepalive is less expensive, but
    # may be inadequate if there is a TCP proxy between Envoy and the management server.
    # HTTP/2 keepalive is slightly more expensive, but may detect issues through more types
    # of intermediate proxies.
    typed_extension_protocol_options:
      envoy.extensions.upstreams.http.v3.HttpProtocolOptions:
        "@type": type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions
        explicit_http_config:
          http2_protocol_options:
            connection_keepalive:
              interval: 30s
              timeout: 5s
    upstream_connection_options:
      tcp_keepalive: {}
`
)

func RunProxyAPIFixtures(ctx context.Context, t *testing.T) {
	var (
		logger   = slog.New(slog.NewTextHandler(os.Stdout, nil))
		grpcServ = grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
		ca       = cache.NewSnapshotCache(true, cache.IDHash{}, nil)
		svc      = proxy.NewService(
			logger,
			proxy.Config{
				DNSUpstream: DNSUpstream,
			}, xds.NewMap("proxy-0", ca),
		)
		proxyServ  = proxy.NewServer(svc)
		envoyImage = os.Getenv("FUNCTESTS_ENVOY_IMAGE")
	)

	proxyv1alpha1.RegisterProxyServiceServer(grpcServ, proxyServ)
	xds.CreateAndRegisterServer(context.Background(), logger, grpcServ, ca)

	require.NoError(t, svc.ApplyGlobalResources(ctx))

	unixSock, err := net.Listen("unix", platformdAddr)
	require.NoError(t, err)

	err = os.Chown(platformdAddr, 9012, 9012)
	require.NoError(t, err)

	t.Cleanup(func() {
		grpcServ.Stop()
		unixSock.Close()
	})
	go func() {
		require.NoError(t, grpcServ.Serve(unixSock))
	}()

	test.WaitServerReady(t, "unix", platformdAddr, 20*time.Second)

	// unix socket must be created, before we start envoy, otherwise we cannot mount it.

	req := testcontainers.ContainerRequest{
		Image: envoyImage,
		ConfigModifier: func(cfg *container.Config) {
			cfg.User = fmt.Sprintf(
				"%s:%s",
				os.Getenv("FUNCTESTS_PLATFORMD_UID"),
				os.Getenv("FUNCTESTS_PLATFORMD_GID"),
			)
		},
		Cmd: []string{
			"-c", "/etc/envoy/config.yaml",
		},
		Files: []testcontainers.ContainerFile{
			{
				Reader:            bytes.NewReader([]byte(envoyCfg)),
				ContainerFilePath: "/etc/envoy/config.yaml",
				FileMode:          0777,
			},
		},
		HostConfigModifier: func(cfg *container.HostConfig) {
			cfg.NetworkMode = "host"
			cfg.AutoRemove = true
			cfg.Mounts = []mount.Mount{
				{
					Type:   mount.TypeBind,
					Source: "/tmp/platformd.sock",
					Target: "/tmp/platformd.sock",
				},
			}
		},
		LogConsumerCfg: &testcontainers.LogConsumerConfig{
			Consumers: []testcontainers.LogConsumer{
				&testcontainers.StdoutLogConsumer{},
			},
		},
	}

	_, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	test.WaitServerReady(t, "tcp", EnvoyAdminAddr, 20*time.Second)
}
