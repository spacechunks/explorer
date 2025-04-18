package xds_test

import (
	"context"
	"testing"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/spacechunks/explorer/internal/mock"
	xds2 "github.com/spacechunks/explorer/platformd/proxy/xds"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestResourceGroupResourcesByType(t *testing.T) {
	rg := xds2.ResourceGroup{
		Clusters: []*clusterv3.Cluster{
			{
				Name: "c1",
			},
			{
				Name: "c2",
			},
		},
		Listeners: []*listenerv3.Listener{
			{
				Name: "l1",
			},
			{
				Name: "l2",
			},
		},
		CLAS: []*endpointv3.ClusterLoadAssignment{
			{
				ClusterName: "c1",
			},
			{
				ClusterName: "c2",
			},
		},
	}

	expected := map[resource.Type][]types.Resource{
		resource.ClusterType:  {rg.Clusters[0], rg.Clusters[1]},
		resource.ListenerType: {rg.Listeners[0], rg.Listeners[1]},
		resource.EndpointType: {rg.CLAS[0], rg.CLAS[1]},
	}

	require.Equal(t, expected, rg.ResourcesByType())
}

func TestMap(t *testing.T) {
	tests := []struct {
		name  string
		check func(xds2.Map, *mock.MockCacheSnapshotCache)
	}{
		{
			name: "check resource group is saved",
			check: func(m xds2.Map, mockCache *mock.MockCacheSnapshotCache) {
				expectedRg := xds2.ResourceGroup{
					Clusters: []*clusterv3.Cluster{
						{
							Name: "c1",
						},
					},
				}

				snap, err := cache.NewSnapshot("1", expectedRg.ResourcesByType())
				require.NoError(t, err)
				mockCache.EXPECT().SetSnapshot(mocky.Anything, mocky.Anything, snap).Return(nil)

				_, err = m.Put(context.Background(), "key", expectedRg)
				require.NoError(t, err)

				require.Equal(t, expectedRg, m.Get("key"))
			},
		},
		{
			name: "all resource groups are merged",
			check: func(m xds2.Map, mockCache *mock.MockCacheSnapshotCache) {
				var (
					merged = make(map[resource.Type][]types.Resource)
					ctx    = context.Background()
					rg1    = xds2.ResourceGroup{
						Clusters: []*clusterv3.Cluster{
							{
								Name: "c1",
							},
						},
						Listeners: []*listenerv3.Listener{
							{
								Name: "l1",
							},
							{
								Name: "l2",
							},
						},
						CLAS: []*endpointv3.ClusterLoadAssignment{
							{
								ClusterName: "c1",
							},
						},
					}
					rg2 = xds2.ResourceGroup{
						Clusters: []*clusterv3.Cluster{
							{
								Name: "c2",
							},
						},
						CLAS: []*endpointv3.ClusterLoadAssignment{
							{
								ClusterName: "c2",
							},
						},
					}
				)

				for _, rg := range []xds2.ResourceGroup{rg1, rg2} {
					for k, v := range rg.ResourcesByType() {
						merged[k] = append(merged[k], v...)
					}
				}

				snap1, err := cache.NewSnapshot("1", rg1.ResourcesByType())
				require.NoError(t, err)

				expectedMerged, err := cache.NewSnapshot("2", merged)
				require.NoError(t, err)

				mockCache.EXPECT().SetSnapshot(mocky.Anything, mocky.Anything, snap1).Return(nil)
				mockCache.EXPECT().SetSnapshot(mocky.Anything, mocky.Anything, expectedMerged).Return(nil)

				_, err = m.Put(ctx, "key1", rg1)
				require.NoError(t, err)

				actualMerged, err := m.Put(ctx, "key2", rg2)
				require.NoError(t, err)

				require.Equal(t, expectedMerged, actualMerged)
			},
		},
		{
			name: "check resource group is deleted",
			check: func(m xds2.Map, mockCache *mock.MockCacheSnapshotCache) {
				var (
					rg = xds2.ResourceGroup{
						Clusters: []*clusterv3.Cluster{
							{
								Name: "c1",
							},
						},
					}
					ctx = context.Background()
					key = "key"
				)

				snap, err := cache.NewSnapshot("1", rg.ResourcesByType())
				require.NoError(t, err)
				mockCache.EXPECT().SetSnapshot(mocky.Anything, mocky.Anything, snap).Return(nil)

				_, err = m.Put(ctx, key, rg)
				require.NoError(t, err)

				delSnap, err := cache.NewSnapshot("2", map[resource.Type][]types.Resource{})
				require.NoError(t, err)
				mockCache.EXPECT().SetSnapshot(mocky.Anything, mocky.Anything, delSnap).Return(nil)

				_, err = m.Del(context.Background(), key)
				require.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				mockCache = mock.NewMockCacheSnapshotCache(t)
				m         = xds2.NewMap("id", mockCache)
			)
			tt.check(m, mockCache)
		})
	}
}
