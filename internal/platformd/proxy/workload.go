package proxy

import (
	"fmt"
	"net/netip"
	"time"

	accesslogv3 "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	httpconnmgr "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/spacechunks/platform/internal/platformd/proxy/xds"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
)

func originalDstClusterResource() *clusterv3.Cluster {
	return &clusterv3.Cluster{
		Name: originalDstClusterName,
		ClusterDiscoveryType: &clusterv3.Cluster_Type{
			Type: clusterv3.Cluster_ORIGINAL_DST,
		},
		ConnectTimeout:  durationpb.New(time.Second * 5),
		DnsLookupFamily: clusterv3.Cluster_V4_ONLY,
		LbPolicy:        clusterv3.Cluster_CLUSTER_PROVIDED,
	}
}

func workloadResources(
	workloadID string,
	httpListenerAddr netip.AddrPort,
	tcpListenerAddr netip.AddrPort,
	originalDstClusterName string,
) (xds.ResourceGroup, error) {
	httpLis, err := httpListener(workloadID, httpListenerAddr, originalDstClusterName)
	if err != nil {
		return xds.ResourceGroup{}, fmt.Errorf("create http listener: %w", err)
	}

	tcpLis, err := tcpListener(workloadID, tcpListenerAddr, originalDstClusterName)
	if err != nil {
		return xds.ResourceGroup{}, fmt.Errorf("create tcp listener: %w", err)
	}

	return xds.ResourceGroup{
		Listeners: []*listenerv3.Listener{httpLis, tcpLis},
	}, nil
}

func tcpListener(workloadID string, addr netip.AddrPort, clusterName string) (*listenerv3.Listener, error) {
	tcpLis, err := xds.TCPProxyListener(xds.ListenerConfig{
		ListenerName: workloadID,
		Addr:         addr,
		Proto:        corev3.SocketAddress_TCP,
	}, xds.TCPProxyConfig{
		StatPrefix:  workloadID,
		ClusterName: clusterName,
	})
	if err != nil {
		return nil, fmt.Errorf("create listener: %w", err)
	}

	dst, err := xds.OriginalDstListenerFilter()
	if err != nil {
		return nil, fmt.Errorf("original dst filter: %v", err)
	}
	tcpLis.ListenerFilters = []*listenerv3.ListenerFilter{dst}

	return tcpLis, nil
}

func httpListener(workloadID string, addr netip.AddrPort, clusterName string) (*listenerv3.Listener, error) {
	httpLis := xds.CreateListener(xds.ListenerConfig{
		ListenerName: workloadID,
		StatPrefix:   workloadID,
		Addr:         addr,
		Proto:        corev3.SocketAddress_TCP,
	})

	httpMgr, err := httpConnenctionManager(workloadID, clusterName)
	if err != nil {
		return nil, fmt.Errorf("create http connenction manager: %w", err)
	}

	var httpMgrAny anypb.Any
	if err := anypb.MarshalFrom(&httpMgrAny, httpMgr, proto.MarshalOptions{}); err != nil {
		return nil, fmt.Errorf("marshal to any: %w", err)
	}

	httpLis.FilterChains = []*listenerv3.FilterChain{
		{
			Filters: []*listenerv3.Filter{
				{
					Name: "envoy.filters.network.http_connection_manager",
					ConfigType: &listenerv3.Filter_TypedConfig{
						TypedConfig: &httpMgrAny,
					},
				},
			},
		},
	}

	dst, err := xds.OriginalDstListenerFilter()
	if err != nil {
		return nil, fmt.Errorf("original dst filter: %v", err)
	}
	httpLis.ListenerFilters = []*listenerv3.ListenerFilter{dst}

	return httpLis, nil
}

func httpConnenctionManager(workloadID string, clusterName string) (*httpconnmgr.HttpConnectionManager, error) {
	alog, err := xds.JSONStdoutAccessLog(nil)
	if err != nil {
		return nil, fmt.Errorf("create access log: %w", err)
	}

	var routerAny anypb.Any
	if err := anypb.MarshalFrom(&routerAny, &routerv3.Router{}, proto.MarshalOptions{}); err != nil {
		return nil, fmt.Errorf("marshal to any: %w", err)
	}

	return &httpconnmgr.HttpConnectionManager{
		StatPrefix: workloadID,
		AccessLog:  []*accesslogv3.AccessLog{alog},
		// do not use RDS here, because those routes will not change for
		// the whole lifecycle of the workload
		RouteSpecifier: &httpconnmgr.HttpConnectionManager_RouteConfig{
			RouteConfig: &routev3.RouteConfiguration{
				Name: "public",
				VirtualHosts: []*routev3.VirtualHost{
					{
						Name:    "all",
						Domains: []string{"*"},
						Routes: []*routev3.Route{
							{
								Match: &routev3.RouteMatch{
									PathSpecifier: &routev3.RouteMatch_Prefix{
										Prefix: "/",
									},
								},
								Action: &routev3.Route_Route{
									Route: &routev3.RouteAction{
										ClusterSpecifier: &routev3.RouteAction_Cluster{
											Cluster: clusterName,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		HttpFilters: []*httpconnmgr.HttpFilter{
			{
				Name: "envoy.filters.http.router",
				ConfigType: &httpconnmgr.HttpFilter_TypedConfig{
					TypedConfig: &routerAny,
				},
			},
		},
	}, nil
}
