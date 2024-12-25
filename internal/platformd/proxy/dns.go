package proxy

import (
	"fmt"
	"net/netip"

	"github.com/spacechunks/platform/internal/platformd/proxy/xds"

	xdscorev3 "github.com/cncf/xds/go/xds/core/v3"
	xsdmatcherv3 "github.com/cncf/xds/go/xds/type/matcher/v3"
	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	udpproxyv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/udp/udp_proxy/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func dnsClusterResource() *clusterv3.Cluster {
	return &clusterv3.Cluster{
		Name: dnsClusterName,
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
}

func dnsListenerResourceGroup(
	clusterName string,
	listenerAddr netip.AddrPort,
	upstreamAddr netip.AddrPort,
) (xds.ResourceGroup, error) {
	udpCLA, udpListener, err := dnsUDPResources(clusterName, listenerAddr, upstreamAddr)
	if err != nil {
		return xds.ResourceGroup{}, fmt.Errorf("udp resources: %w", err)
	}
	tcpCLA, tcpListener, err := dnsTCPResources(clusterName, listenerAddr, upstreamAddr)
	if err != nil {
		return xds.ResourceGroup{}, fmt.Errorf("tcp resources: %w", err)
	}

	return xds.ResourceGroup{
		Listeners: []*listenerv3.Listener{udpListener, tcpListener},
		CLAS:      []*endpointv3.ClusterLoadAssignment{udpCLA, tcpCLA},
	}, nil
}

func dnsTCPResources(clusterName string, listenerAddr, upstreamAddr netip.AddrPort) (
	*endpointv3.ClusterLoadAssignment,
	*listenerv3.Listener,
	error,
) {
	l, err := xds.TCPProxyListener(xds.ListenerConfig{
		ListenerName: "dns_tcp",
		Addr:         listenerAddr,
		Proto:        corev3.SocketAddress_TCP,
	}, xds.TCPProxyConfig{
		StatPrefix:  "dns_tcp_proxy",
		ClusterName: clusterName,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create tcp proxy listener: %w", err)
	}

	return xds.CreateCLA(clusterName, upstreamAddr, corev3.SocketAddress_UDP), l, nil
}

func dnsUDPResources(clusterName string, listenerAddr, upstreamAddr netip.AddrPort) (
	*endpointv3.ClusterLoadAssignment,
	*listenerv3.Listener,
	error,
) {
	dnsRoute := &udpproxyv3.Route{
		Cluster: clusterName,
	}

	var dnsRouteAny anypb.Any
	if err := anypb.MarshalFrom(&dnsRouteAny, dnsRoute, proto.MarshalOptions{}); err != nil {
		return nil, nil, fmt.Errorf("route to any: %w", err)
	}

	filterCfg := &udpproxyv3.UdpProxyConfig{
		StatPrefix: "dns_udp_proxy",
		RouteSpecifier: &udpproxyv3.UdpProxyConfig_Matcher{
			Matcher: &xsdmatcherv3.Matcher{
				OnNoMatch: &xsdmatcherv3.Matcher_OnMatch{
					OnMatch: &xsdmatcherv3.Matcher_OnMatch_Action{
						Action: &xdscorev3.TypedExtensionConfig{
							Name:        "route",
							TypedConfig: &dnsRouteAny,
						},
					},
				},
			},
		},
		UpstreamSocketConfig: &corev3.UdpSocketConfig{
			MaxRxDatagramSize: wrapperspb.UInt64(9000),
		},
	}

	var filterCfgAny anypb.Any
	if err := anypb.MarshalFrom(&filterCfgAny, filterCfg, proto.MarshalOptions{}); err != nil {
		return nil, nil, fmt.Errorf("filter to any: %w", err)
	}

	l := xds.CreateListener(xds.ListenerConfig{
		ListenerName: "dns_udp",
		Addr:         listenerAddr,
		Proto:        corev3.SocketAddress_UDP,
	})

	l.ListenerFilters = []*listenerv3.ListenerFilter{
		{
			Name: "envoy.filters.udp_listener.udp_proxy",
			ConfigType: &listenerv3.ListenerFilter_TypedConfig{
				TypedConfig: &filterCfgAny,
			},
		},
	}

	l.UdpListenerConfig = &listenerv3.UdpListenerConfig{
		DownstreamSocketConfig: &corev3.UdpSocketConfig{
			MaxRxDatagramSize: wrapperspb.UInt64(9000),
		},
	}

	return xds.CreateCLA(clusterName, upstreamAddr, corev3.SocketAddress_UDP), l, nil
}
