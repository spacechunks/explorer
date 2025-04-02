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

package fixture

import (
	"net"
	"testing"

	workloadv1alpha2 "github.com/spacechunks/explorer/api/platformd/workload/v1alpha2"
	"github.com/spacechunks/explorer/internal/ptr"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func Workload() workloadv1alpha2.Workload {
	return workloadv1alpha2.Workload{
		Name:                 ptr.Pointer("my-chunk"),
		BaseImageUrl:         ptr.Pointer("my-image"),
		Namespace:            ptr.Pointer("chunk-ns"),
		Hostname:             ptr.Pointer("my-chunk"),
		Labels:               map[string]string{"k": "v"},
		NetworkNamespaceMode: ptr.Pointer(int32(2)),
	}
}

func RunWorkloadAPIFixtures(t *testing.T) {
	var (
		criServ = grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	)

	criSock, err := net.Listen("unix", "@"+platformdAddr)
	require.NoError(t, err)

	cri := newFakeCRI()

	runtimev1.RegisterRuntimeServiceServer(criServ, cri)
	runtimev1.RegisterImageServiceServer(criServ, cri)

	//workloadv1alpha2.RegisterWorkloadServiceServer(criServ, wlServ)

	t.Cleanup(func() {
		criServ.Stop()
		criSock.Close()
	})

	go func() {
		err = criServ.Serve(criSock)
		require.NoError(t, err)
	}()
}
