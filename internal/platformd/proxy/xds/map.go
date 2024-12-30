package xds

import (
	"context"
	"fmt"
	"sync"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
)

// ResourceGroup should be used for grouping related resources.
// For example all resources related to a DNS proxy should be
// placed into one specific instance of a ResourceGroup.
type ResourceGroup struct {
	Clusters  []*clusterv3.Cluster
	Listeners []*listenerv3.Listener
	CLAS      []*endpointv3.ClusterLoadAssignment
}

// ResourcesByType returns a map that can be used directly when applying
// a new snapshot.
func (rg *ResourceGroup) ResourcesByType() map[resource.Type][]types.Resource {
	m := make(map[resource.Type][]types.Resource)
	for _, c := range rg.Clusters {
		m[resource.ClusterType] = append(m[resource.ClusterType], c)
	}
	for _, l := range rg.Listeners {
		m[resource.ListenerType] = append(m[resource.ListenerType], l)
	}
	for _, cla := range rg.CLAS {
		m[resource.EndpointType] = append(m[resource.EndpointType], cla)
	}
	return m
}

// Map is responsible for holding and applying envoy configuration resources.
// This is necessary, because when applying new resources all previous ones that
// are not contained in the snapshot will be removed by envoy. This map is safe
// for concurrent use.
type Map struct {
	cache     cache.SnapshotCache
	mu        sync.Mutex
	resources map[string]ResourceGroup
	nodeID    string
	version   uint64
}

func NewMap(nodeID string, cache cache.SnapshotCache) *Map {
	return &Map{
		cache:     cache,
		resources: make(map[string]ResourceGroup),
		nodeID:    nodeID,
	}
}

func (m *Map) Get(key string) ResourceGroup {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.resources[key]
}

// Apply saves the passed resource group in the map under the provided
// key, creates a new snapshot and applies it to all known envoy nodes at the time.
// Returns the applied snapshot.
func (m *Map) Apply(ctx context.Context, key string, rg ResourceGroup) (*cache.Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.resources[key] = rg
	typeToRes := make(map[resource.Type][]types.Resource)

	// merge all resources from all resource groups to
	// get a complete view of the envoy configuration
	// to apply.
	for _, v := range m.resources {
		for typ, res := range v.ResourcesByType() {
			typeToRes[typ] = append(typeToRes[typ], res...)
		}
	}

	m.version++

	snap, err := cache.NewSnapshot(fmt.Sprintf("%d", m.version), typeToRes)
	if err != nil {
		return nil, fmt.Errorf("create snapshot: %w", err)
	}

	if err := m.cache.SetSnapshot(ctx, m.nodeID, snap); err != nil {
		return nil, fmt.Errorf("set snapshot: %w", err)
	}

	return snap, nil
}
