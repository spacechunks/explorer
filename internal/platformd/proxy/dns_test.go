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
	"log"
	"net/netip"
	"testing"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	"github.com/google/go-cmp/cmp"
	"github.com/spacechunks/platform/internal/platformd/proxy"
	"github.com/spacechunks/platform/internal/platformd/proxy/xds"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestDNSClusterConfig(t *testing.T) {
	var (
		expected = &clusterv3.Cluster{
			Name: proxy.DNSClusterName,
			ClusterDiscoveryType: &clusterv3.Cluster_Type{
				Type: clusterv3.Cluster_EDS,
			},
			EdsClusterConfig: &clusterv3.Cluster_EdsClusterConfig{
				EdsConfig: &corev3.ConfigSource{
					ConfigSourceSpecifier: &corev3.ConfigSource_Ads{},
				},
			},
			LbPolicy: clusterv3.Cluster_ROUND_ROBIN,
		}
		actual = proxy.DNSClusterResource()
	)

	d := cmp.Diff(expected, actual, protocmp.Transform())
	if d != "" {
		t.Fatal(d)
	}
}

func TestDNSResourceGroupConfig(t *testing.T) {
	var (
		clusterName  = "test-dns"
		listenerAddr = netip.MustParseAddrPort("127.0.0.1:9053")
		upstreamAddr = netip.MustParseAddrPort("127.0.0.3:53")

		expectedUDPListener = fmt.Sprintf(`
{
  "name":  "dns_udp",
  "address":  {
    "socketAddress":  {
      "protocol": "UDP",
      "address": "%s",
      "portValue": %d
    }
  },
  "listenerFilters":  [
    {
      "name": "envoy.filters.udp_listener.udp_proxy",
      "typedConfig":  {
        "@type": "type.googleapis.com/envoy.extensions.filters.udp.udp_proxy.v3.UdpProxyConfig",
        "statPrefix":  "dns_udp_proxy",
        "matcher": {
          "onNoMatch": {
            "action": {
              "name": "route",
              "typedConfig": {
                "@type": "type.googleapis.com/envoy.extensions.filters.udp.udp_proxy.v3.Route",
                "cluster": "test-dns"
              }
            }
          }
        },
        "upstreamSocketConfig":  {
          "maxRxDatagramSize": "9000"
        }
      }
    }
  ],
  "udpListenerConfig":  {
    "downstreamSocketConfig":  {
      "maxRxDatagramSize": "9000"
    }
  }
}`, listenerAddr.Addr().String(), listenerAddr.Port())

		expectedTCPListener = fmt.Sprintf(`
{
  "name":  "dns_tcp",
  "address":  {
    "socketAddress":  {
      "address": "%s",
      "portValue": %d
    }
  },
  "filterChains":  [
    {
      "filters":  [
        {
          "name":  "envoy.filters.network.tcp_proxy",
          "typedConfig":  {
            "@type":  "type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy",
            "statPrefix":  "dns_tcp_proxy",
            "cluster":  "test-dns"
          }
        }
      ]
    }
  ]
}`, listenerAddr.Addr().String(), listenerAddr.Port())

		expectedUDPCLA = fmt.Sprintf(`
{
  "clusterName": "test-dns",
  "endpoints": [
    {
      "lbEndpoints": [
        {
          "endpoint": {
            "address": {
              "socketAddress": {
                "protocol": "UDP",
                "address": "%s",
                "portValue": %d
              }
            }
          }
        }
      ]
    }
  ]
}`, upstreamAddr.Addr().String(), upstreamAddr.Port())

		expectedTCPCLA = fmt.Sprintf(`
{
  "clusterName": "test-dns",
  "endpoints": [
    {
      "lbEndpoints": [
        {
          "endpoint": {
            "address": {
              "socketAddress": {
                "address": "%s",
                "portValue": %d
              }
            }
          }
        }
      ]
    }
  ]
}`, upstreamAddr.Addr().String(), upstreamAddr.Port())
	)

	udpLis := &listenerv3.Listener{}
	require.NoError(t, protojson.Unmarshal([]byte(expectedUDPListener), udpLis))

	tcpLis := &listenerv3.Listener{}
	require.NoError(t, protojson.Unmarshal([]byte(expectedTCPListener), tcpLis))

	udpCLA := &endpointv3.ClusterLoadAssignment{}
	require.NoError(t, protojson.Unmarshal([]byte(expectedUDPCLA), udpCLA))

	tcpCLA := &endpointv3.ClusterLoadAssignment{}
	require.NoError(t, protojson.Unmarshal([]byte(expectedTCPCLA), tcpCLA))

	expectedRG := xds.ResourceGroup{
		Listeners: []*listenerv3.Listener{udpLis, tcpLis},
		CLAS:      []*endpointv3.ClusterLoadAssignment{udpCLA, tcpCLA},
	}

	actualRG, err := proxy.DNSListenerResourceGroup(clusterName, listenerAddr, upstreamAddr)
	require.NoError(t, err)

	d := cmp.Diff(expectedRG, actualRG, protocmp.Transform())
	if d != "" {
		log.Fatal(d)
	}
}
