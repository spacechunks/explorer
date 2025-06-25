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

package functional

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"sort"
	"testing"
	"time"

	adminv3 "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	"github.com/google/go-cmp/cmp"
	proxyv1alpha1 "github.com/spacechunks/explorer/api/platformd/proxy/v1alpha1"
	"github.com/spacechunks/explorer/platformd/proxy"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestCreateListener(t *testing.T) {
	var (
		ctx  = context.Background()
		wlID = "abc"
		ip   = "127.0.0.1"
	)

	fixture.RunProxyAPIFixtures(ctx, t)

	c := proxyv1alpha1.NewProxyServiceClient(fixture.PlatformdClientConn(t))

	_, err := c.CreateListeners(ctx, &proxyv1alpha1.CreateListenersRequest{
		WorkloadID: wlID,
		Ip:         ip,
	})
	require.NoError(t, err)

	// FIXME(yannic): implement some sort of WaitReady function into
	//                proxy package, that blocks until envoy has connected.
	time.Sleep(10 * time.Second)

	dnsRG, err := proxy.DNSListenerResourceGroup(
		proxy.DNSClusterName,
		netip.MustParseAddrPort(fmt.Sprintf("%s:%d", ip, proxy.DNSPort)),
		fixture.DNSUpstream,
	)
	require.NoError(t, err)

	wlRG, err := proxy.WorkloadResources(wlID,
		netip.MustParseAddrPort(fmt.Sprintf("%s:%d", ip, proxy.HTTPPort)),
		netip.MustParseAddrPort(fmt.Sprintf("%s:%d", ip, proxy.TCPPort)),
		proxy.OriginalDstClusterName,
	)
	require.NoError(t, err)

	var (
		actual   = readListener(t)
		expected = append(dnsRG.Listeners, wlRG.Listeners...)
	)

	// we have to sort both arrays, otherwise the Diff later
	// will fail, because items in the slices are not in the ´
	// same order.

	sort.Slice(actual, func(i, j int) bool {
		return actual[i].Name < actual[j].Name
	})
	sort.Slice(expected, func(i, j int) bool {
		return expected[i].Name < expected[j].Name
	})

	d := cmp.Diff(expected, actual, protocmp.Transform())
	if d != "" {
		t.Fatal(d)
	}
}

func TestDeleteListener(t *testing.T) {
	var (
		ctx  = context.Background()
		wlID = "abc"
		ip   = "127.0.0.1"
	)

	fixture.RunProxyAPIFixtures(ctx, t)

	c := proxyv1alpha1.NewProxyServiceClient(fixture.PlatformdClientConn(t))

	_, err := c.CreateListeners(ctx, &proxyv1alpha1.CreateListenersRequest{
		WorkloadID: wlID,
		Ip:         ip,
	})
	require.NoError(t, err)

	_, err = c.DeleteListeners(ctx, &proxyv1alpha1.DeleteListenersRequest{
		WorkloadID: wlID,
	})
	require.NoError(t, err)

	// FIXME(yannic): implement some sort of WaitReady function into
	//                proxy package, that blocks until envoy has connected.
	time.Sleep(10 * time.Second)

	actual := readListener(t)

	d := cmp.Diff([]*listenerv3.Listener{}, actual, protocmp.Transform())
	if d != "" {
		t.Fatal(d)
	}
}

func readListener(t *testing.T) []*listenerv3.Listener {
	resp, err := http.Get(
		fmt.Sprintf("http://%s/config_dump?include_eds&resource=dynamic_listeners", fixture.EnvoyAdminAddr),
	)
	require.NoError(t, err)

	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	payload := struct {
		// configs is a list of listenerv3.Listener
		Configs []json.RawMessage `json:"configs"`
	}{}

	err = json.Unmarshal(data, &payload)
	require.NoError(t, err)

	ret := make([]*listenerv3.Listener, 0)

	unmarshal := protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}

	for _, cfg := range payload.Configs {
		dyn := adminv3.ListenersConfigDump_DynamicListener{}
		require.NoError(t, unmarshal.Unmarshal(cfg, &dyn))

		// handle scenario where an empty config ListenersConfigDump_DynamicListener
		// is returned. this is the case for TestDeleteListener. the returned JSON
		// by config_dump endpoint is then [{}] i.e a list with a single empty object.
		if dyn.ActiveState == nil {
			continue
		}

		lis := listenerv3.Listener{}
		err = anypb.UnmarshalTo(dyn.ActiveState.Listener, &lis, proto.UnmarshalOptions{
			Merge:          true,
			DiscardUnknown: true,
		})
		require.NoError(t, err)

		ret = append(ret, &lis)
	}
	return ret
}
