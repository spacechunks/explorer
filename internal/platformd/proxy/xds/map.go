package xds

import (
	"context"
	"fmt"
	"sync"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
)

// Map is responsible for storing and applying envoy configuration resources.
// This is necessary, because when applying new resources all previous ones that
// are not contained in the snapshot will be removed by envoy. Implementations
// must be safe for concurrent use.
type Map interface {
	Get(key string) ResourceGroup
	Apply(ctx context.Context, key string, rg ResourceGroup) (*cache.Snapshot, error)
}

type inmemMap struct {
	cache     cache.SnapshotCache
	mu        sync.Mutex
	resources map[string]ResourceGroup
	nodeID    string
	version   uint64
}

func NewMap(nodeID string, cache cache.SnapshotCache) Map {
	return &inmemMap{
		cache:     cache,
		resources: make(map[string]ResourceGroup),
		nodeID:    nodeID,
	}
}

func (m *inmemMap) Get(key string) ResourceGroup {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.resources[key]
}

// Apply saves the passed resource group in the map under the provided
// key, creates a new snapshot and applies it to all known envoy nodes at the time.
// Returns the applied snapshot.
func (m *inmemMap) Apply(ctx context.Context, key string, rg ResourceGroup) (*cache.Snapshot, error) {
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
