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

package proxy_test

import (
	"fmt"
	"net/netip"
	"testing"

	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	"github.com/google/go-cmp/cmp"
	proxy2 "github.com/spacechunks/explorer/platformd/proxy"
	"github.com/spacechunks/explorer/platformd/proxy/xds"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestWorkloadResourceGroupConfig(t *testing.T) {
	var (
		wlID             = "abc"
		httpListenerAddr = netip.MustParseAddrPort(fmt.Sprintf("127.0.0.1:%d", proxy2.HTTPPort))
		tcpListenerAddr  = netip.MustParseAddrPort(fmt.Sprintf("127.0.0.1:%d", proxy2.TCPPort))

		expectedTCPListener = fmt.Sprintf(`
{
  "name": "tcp-%s",
  "address": {
    "socketAddress": {
      "address": "%s",
      "portValue": %d
    }
  },
  "filterChains": [
    {
      "filters": [
        {
          "name": "envoy.filters.network.tcp_proxy",
          "typedConfig": {
            "@type": "type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy",
            "statPrefix": "%s",
            "cluster": "%s"
          }
        }
      ]
    }
  ],
  "listenerFilters": [
    {
      "name": "envoy.filters.listener.original_dst",
      "typedConfig": {
        "@type": "type.googleapis.com/envoy.extensions.filters.listener.original_dst.v3.OriginalDst"
      }
    }
  ]
}`, wlID, tcpListenerAddr.Addr().String(), tcpListenerAddr.Port(), wlID, proxy2.OriginalDstClusterName)

		//nolint:lll
		expectedHTTPListener = fmt.Sprintf(`
{
  "name": "http-%s",
  "address": {
    "socketAddress": {
      "address": "%s",
      "portValue": %d
    }
  },
  "statPrefix": "%s",
  "filterChains": [
    {
      "filters": [
        {
          "name": "envoy.filters.network.http_connection_manager",
          "typedConfig": {
            "@type": "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager",
            "statPrefix": "%s",
            "routeConfig": {
              "name": "public",
              "virtualHosts": [
                {
                  "name": "all",
                  "domains": [
                    "*"
                  ],
                  "routes": [
                    {
                      "match": {
                        "prefix": "/"
                      },
                      "route": {
                        "cluster": "%s"
                      }
                    }
                  ]
                }
              ]
            },
            "httpFilters": [
              {
                "name": "envoy.filters.http.router",
                "typedConfig": {
                  "@type": "type.googleapis.com/envoy.extensions.filters.http.router.v3.Router"
                }
              }
            ],
            "accessLog": [
              {
                "name": "json_stdout_access_log",
                "typedConfig": {
                  "@type": "type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog",
                  "logFormat": {
                    "jsonFormat": {},
                    "omitEmptyValues": true,
                    "jsonFormatOptions": {
                      "sortProperties": true
                    }
                  }
                }
              }
            ]
          }
        }
      ]
    }
  ],
  "listenerFilters": [
    {
      "name": "envoy.filters.listener.original_dst",
      "typedConfig": {
        "@type": "type.googleapis.com/envoy.extensions.filters.listener.original_dst.v3.OriginalDst"
      }
    }
  ]
}`, wlID, httpListenerAddr.Addr().String(), httpListenerAddr.Port(), wlID, wlID, proxy2.OriginalDstClusterName)
	)

	tcpLis := &listenerv3.Listener{}
	require.NoError(t, protojson.Unmarshal([]byte(expectedTCPListener), tcpLis))

	httpLis := &listenerv3.Listener{}
	require.NoError(t, protojson.Unmarshal([]byte(expectedHTTPListener), httpLis))

	expectedRG := xds.ResourceGroup{
		Listeners: []*listenerv3.Listener{tcpLis, httpLis},
	}

	actualRG, err := proxy2.WorkloadResources(wlID, httpListenerAddr, tcpListenerAddr, proxy2.OriginalDstClusterName)
	require.NoError(t, err)

	d := cmp.Diff(expectedRG, actualRG, protocmp.Transform())
	if d != "" {
		t.Fatal(d)
	}
}
