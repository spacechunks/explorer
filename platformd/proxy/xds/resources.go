package xds

import (
	"fmt"
	"net/netip"

	accesslogv3 "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	streamv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/stream/v3"
	originaldstv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/listener/original_dst/v3"
	tcpproxyv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

type ListenerConfig struct {
	ListenerName string
	StatPrefix   string
	Addr         netip.AddrPort
	Proto        corev3.SocketAddress_Protocol
}

// CreateListener creates a listener with a single listener filter config
func CreateListener(cfg ListenerConfig) *listenerv3.Listener {
	return &listenerv3.Listener{
		Name: cfg.ListenerName,
		Address: &corev3.Address{
			Address: &corev3.Address_SocketAddress{
				SocketAddress: &corev3.SocketAddress{
					Protocol: cfg.Proto,
					Address:  cfg.Addr.Addr().String(),
					PortSpecifier: &corev3.SocketAddress_PortValue{
						PortValue: uint32(cfg.Addr.Port()),
					},
				},
			},
		},
		StatPrefix: cfg.StatPrefix,
	}
}

// CreateCLA creates a cluster load assignment with a single upstream endpoint
func CreateCLA(
	clusterName string,
	upstream netip.AddrPort,
	proto corev3.SocketAddress_Protocol,
) *endpointv3.ClusterLoadAssignment {
	return &endpointv3.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints: []*endpointv3.LocalityLbEndpoints{
			{
				LbEndpoints: []*endpointv3.LbEndpoint{
					{
						HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
							Endpoint: &endpointv3.Endpoint{
								Address: &corev3.Address{
									Address: &corev3.Address_SocketAddress{
										SocketAddress: &corev3.SocketAddress{
											Protocol: proto,
											Address:  upstream.Addr().String(),
											PortSpecifier: &corev3.SocketAddress_PortValue{
												PortValue: uint32(upstream.Port()),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func JSONStdoutAccessLog(format map[string]any) (*accesslogv3.AccessLog, error) {
	jsonFormat, err := structpb.NewStruct(format)
	if err != nil {
		return nil, fmt.Errorf("new json format struct: %w", err)
	}

	stdout := &streamv3.StdoutAccessLog{
		AccessLogFormat: &streamv3.StdoutAccessLog_LogFormat{
			LogFormat: &corev3.SubstitutionFormatString{
				Format: &corev3.SubstitutionFormatString_JsonFormat{
					JsonFormat: jsonFormat,
				},
				OmitEmptyValues: true,
				JsonFormatOptions: &corev3.JsonFormatOptions{
					SortProperties: true,
				},
			},
		},
	}

	var stdoutAny anypb.Any
	if err := anypb.MarshalFrom(&stdoutAny, stdout, proto.MarshalOptions{}); err != nil {
		return nil, fmt.Errorf("marshal to any: %w", err)
	}

	return &accesslogv3.AccessLog{
		Name: "json_stdout_access_log",
		ConfigType: &accesslogv3.AccessLog_TypedConfig{
			TypedConfig: &stdoutAny,
		},
	}, nil
}

type TCPProxyConfig struct {
	StatPrefix  string
	ClusterName string
}

func TCPProxyListener(listenerCfg ListenerConfig, proxyCfg TCPProxyConfig) (*listenerv3.Listener, error) {
	filterCfg := &tcpproxyv3.TcpProxy{
		StatPrefix: proxyCfg.StatPrefix,
		ClusterSpecifier: &tcpproxyv3.TcpProxy_Cluster{
			Cluster: proxyCfg.ClusterName,
		},
	}
	var filterCfgAny anypb.Any
	if err := anypb.MarshalFrom(&filterCfgAny, filterCfg, proto.MarshalOptions{}); err != nil {
		return nil, fmt.Errorf("marshal to any: %w", err)
	}
	l := CreateListener(listenerCfg)
	l.FilterChains = []*listenerv3.FilterChain{
		{
			Filters: []*listenerv3.Filter{
				{
					Name: "envoy.filters.network.tcp_proxy",
					ConfigType: &listenerv3.Filter_TypedConfig{
						TypedConfig: &filterCfgAny,
					},
				},
			},
		},
	}
	return l, nil
}

func OriginalDstListenerFilter() (*listenerv3.ListenerFilter, error) {
	var origDstAny anypb.Any
	if err := anypb.MarshalFrom(&origDstAny, &originaldstv3.OriginalDst{}, proto.MarshalOptions{}); err != nil {
		return nil, fmt.Errorf("marshal to any: %w", err)
	}
	return &listenerv3.ListenerFilter{
		Name: "envoy.filters.listener.original_dst",
		ConfigType: &listenerv3.ListenerFilter_TypedConfig{
			TypedConfig: &origDstAny,
		},
	}, nil
}
