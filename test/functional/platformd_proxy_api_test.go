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
	"io"
	"log"
	"net/http"
	"testing"

	proxyv1alpha1 "github.com/spacechunks/platform/api/platformd/proxy/v1alpha1"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestPlatformdProxyAPICreateListener(t *testing.T) {
	ctx := context.Background()
	runProxyFixture(ctx, t)

	conn, err := grpc.NewClient("unix-abstract:"+proxyAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	defer conn.Close()

	c := proxyv1alpha1.NewProxyServiceClient(conn)

	_, err = c.CreateListeners(ctx, &proxyv1alpha1.CreateListenersRequest{
		WorkloadID: "abcv",
		Ip:         "127.0.0.1",
	})
	require.NoError(t, err)

	resp, err := http.Get("http://127.0.0.1:5555/config_dump?include_eds")
	require.NoError(t, err)

	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	//var j map[string]any
	//require.NoError(t, json.Unmarshal(data, &j))

	log.Println(string(data))
}
