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
	"context"
	"log/slog"
	"net/netip"
	"os"
	"testing"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	"github.com/spacechunks/platform/internal/mock"
	"github.com/spacechunks/platform/internal/platformd/proxy"
	"github.com/spacechunks/platform/internal/platformd/proxy/xds"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestApplyGlobalResources(t *testing.T) {
	var (
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
		rg     = xds.ResourceGroup{
			Clusters: []*clusterv3.Cluster{
				proxy.DNSClusterResource(),
				proxy.OriginalDstClusterResource(),
			},
		}
		mockMap = mock.NewMockXdsMap(t)
		svc     = proxy.NewService(logger, proxy.Config{}, mockMap)
	)

	mockMap.EXPECT().Put(mocky.Anything, "global", rg).Return(nil, nil)
	require.NoError(t, svc.ApplyGlobalResources(context.Background()))
}

func TestCreateListeners(t *testing.T) {
	var (
		wlID        = "abc"
		addr        = netip.MustParseAddr("127.0.0.1")
		dnsUpstream = netip.MustParseAddrPort("127.0.0.1:53")
	)

	wrg, err := proxy.WorkloadResources(
		wlID,
		netip.AddrPortFrom(addr, proxy.HTTPPort),
		netip.AddrPortFrom(addr, proxy.TCPPort),
		proxy.OriginalDstClusterName,
	)
	require.NoError(t, err)

	drg, err := proxy.DNSListenerResourceGroup(
		proxy.DNSClusterName,
		netip.AddrPortFrom(addr, proxy.DNSPort),
		dnsUpstream,
	)
	require.NoError(t, err)

	merged := xds.ResourceGroup{}
	merged.Listeners = append(wrg.Listeners, drg.Listeners...)
	merged.Clusters = append(wrg.Clusters, drg.Clusters...)
	merged.CLAS = append(wrg.CLAS, drg.CLAS...)

	var (
		ctx     = context.Background()
		mockMap = mock.NewMockXdsMap(t)
		logger  = slog.New(slog.NewTextHandler(os.Stdout, nil))
		svc     = proxy.NewService(logger, proxy.Config{
			DNSUpstream: dnsUpstream,
		}, mockMap)
	)

	mockMap.EXPECT().Put(mocky.Anything, wlID, merged).Return(nil, nil)
	require.NoError(t, svc.CreateListeners(ctx, wlID, addr))
}

func TestDeleteListeners(t *testing.T) {
	var (
		ctx     = context.Background()
		mockMap = mock.NewMockXdsMap(t)
		logger  = slog.New(slog.NewTextHandler(os.Stdout, nil))
		svc     = proxy.NewService(logger, proxy.Config{
			DNSUpstream: netip.MustParseAddrPort("127.0.0.1:53"),
		}, mockMap)
		wlID = "abc"
	)
	mockMap.EXPECT().Del(mocky.Anything, wlID).Return(nil, nil)
	require.NoError(t, svc.DeleteListeners(ctx, wlID))
}
