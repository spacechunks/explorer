package xds

import (
	"context"
	"log/slog"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	serverv3 "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"google.golang.org/grpc"
)

func CreateAndRegisterServer(ctx context.Context, logger *slog.Logger, server *grpc.Server, cache cache.SnapshotCache) {
	xdsServer := serverv3.NewServer(ctx, cache, serverv3.CallbackFuncs{
		StreamOpenFunc: func(ctx context.Context, _ int64, _ string) error {
			logger.InfoContext(ctx, "connected to proxy")
			return nil
		},
		StreamClosedFunc: func(i int64, node *corev3.Node) {
			logger.InfoContext(ctx, "connection to proxy closed")
		},
	})
	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(server, xdsServer)
}
