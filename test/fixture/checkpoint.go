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
	"time"

	checkpointv1alpha1 "github.com/spacechunks/explorer/api/platformd/checkpoint/v1alpha1"
	"github.com/spacechunks/explorer/internal/image"
	"github.com/spacechunks/explorer/platformd/checkpoint"
	"github.com/spacechunks/explorer/platformd/cri"
	"github.com/spacechunks/explorer/platformd/status"
	"github.com/spacechunks/explorer/platformd/workload"
	"github.com/spacechunks/explorer/test"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/client-go/tools/remotecommand"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

var CheckpointAPIAddr = "127.0.0.1:3012"

func RunCheckpointAPIFixtures(t *testing.T, registryUser string, registryPass string) {
	criConn, err := grpc.NewClient(
		"unix-abstract:"+FakeCRIAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	var (
		logger   = slog.New(slog.NewTextHandler(os.Stdout, nil)).With("service", "checkpoint-api")
		grpcServ = grpc.NewServer(grpc.Creds(insecure.NewCredentials()))

		rtClient  = runtimev1.NewRuntimeServiceClient(criConn)
		imgClient = runtimev1.NewImageServiceClient(criConn)

		svc = checkpoint.NewService(
			logger,
			checkpoint.Config{
				CPUPeriod:                100,
				CPUQuota:                 200,
				MemoryLimitBytes:         300,
				CheckpointFileDir:        t.TempDir(),
				CheckpointTimeoutSeconds: 60,
				//RegistryUser:             registryUser,
				//RegistryPass:             registryPass,
				ListenAddr:            CheckpointAPIAddr,
				StatusRetentionPeriod: 10 * time.Second,
				ContainerReadyTimeout: 5 * time.Second,
			},
			cri.NewService(logger.With("component", "cri-service"), rtClient, imgClient),
			image.NewService(logger.With("component", "image-service"), registryUser, registryPass, t.TempDir()),
			status.NewMemStore(),
			func(url string) (remotecommand.Executor, error) {
				return &test.RemoteCmdExecutor{}, nil
			},
			workload.NewPortAllocator(5000, 6000),
		)
		checkServ = checkpoint.NewServer(svc)
	)

	checkpointv1alpha1.RegisterCheckpointServiceServer(grpcServ, checkServ)

	l, err := net.Listen("tcp", CheckpointAPIAddr)
	require.NoError(t, err)
	t.Cleanup(func() {
		grpcServ.Stop()
		l.Close()
		criConn.Close()
	})
	go func() {
		require.NoError(t, grpcServ.Serve(l))
	}()

	test.WaitServerReady(t, "tcp", CheckpointAPIAddr, 20*time.Second)
}
