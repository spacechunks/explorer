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
	"log/slog"
	"net"
	"os"
	"testing"

	workloadv1alpha1 "github.com/spacechunks/explorer/api/platformd/workload/v1alpha1"
	"github.com/spacechunks/explorer/internal/platformd/workload"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func Workload() workloadv1alpha1.Workload {
	return workloadv1alpha1.Workload{
		Name:                 "my-chunk",
		Image:                "my-image",
		Namespace:            "chunk-ns",
		Hostname:             "my-chunk",
		Labels:               map[string]string{"k": "v"},
		NetworkNamespaceMode: 2,
	}
}

func RunWorkloadAPIFixtures(t *testing.T) {
	var (
		logger  = slog.New(slog.NewTextHandler(os.Stdout, nil))
		criServ = grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	)

	criSock, err := net.Listen("unix", "@"+platformdAddr)
	require.NoError(t, err)

	cri := newFakeCRI()

	runtimev1.RegisterRuntimeServiceServer(criServ, cri)
	runtimev1.RegisterImageServiceServer(criServ, cri)

	conn := PlatformdClientConn(t)

	wlServ := workload.NewServer(
		workload.NewService(
			logger,
			runtimev1.NewRuntimeServiceClient(conn),
			runtimev1.NewImageServiceClient(conn),
		),
		workload.NewPortAllocator(20, 50),
		workload.NewStore(),
	)

	workloadv1alpha1.RegisterWorkloadServiceServer(criServ, wlServ)

	t.Cleanup(func() {
		criServ.Stop()
		criSock.Close()
	})

	go func() {
		err = criServ.Serve(criSock)
		require.NoError(t, err)
	}()
}
