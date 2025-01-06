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

package xds

import (
	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
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
