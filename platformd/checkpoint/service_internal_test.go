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

package checkpoint

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spacechunks/explorer/internal/mock"
	"github.com/spacechunks/explorer/platformd/cri"
	"github.com/spacechunks/explorer/platformd/status"
	"github.com/spacechunks/explorer/platformd/workload"
	"github.com/spacechunks/explorer/test"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/remotecommand"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type prepArgs struct {
	svc        *ServiceImpl
	mockCRISvc *mock.MockCriService
	mockImgSvc *mock.MockImageService
	cfg        Config
	checkID    string
	podID      string
	ctrID      string
	baseRef    name.Reference
}

func TestCheckpoint(t *testing.T) {
	tests := []struct {
		name  string
		cfg   Config
		err   error
		state status.CheckpointState
		prep  func(args prepArgs)
	}{
		{
			name: "works",
			cfg: Config{
				CPUPeriod:                100,
				CPUQuota:                 200,
				MemoryLimitBytes:         300,
				CheckpointFileDir:        t.TempDir(),
				CheckpointTimeoutSeconds: 60,
				ContainerReadyTimeout:    10 * time.Second,
				RegistryUser:             "user",
				RegistryPass:             "pass",
			},
			state: status.CheckpointStateCompleted,
			prep: func(args prepArgs) {
				prepUntilContainerAttach(
					args.svc,
					args.checkID,
					args.podID,
					args.ctrID,
					args.mockCRISvc,
					args.baseRef,
					cri.RegistryAuth{
						Username: args.cfg.RegistryUser,
						Password: args.cfg.RegistryPass,
					},
				)

				fileLoc := fmt.Sprintf("%s/%s", args.cfg.CheckpointFileDir, args.checkID)

				args.mockCRISvc.EXPECT().
					CheckpointContainer(mocky.Anything, &runtimev1.CheckpointContainerRequest{
						ContainerId: args.ctrID,
						Location:    fileLoc,
						Timeout:     args.cfg.CheckpointTimeoutSeconds,
					}).
					Return(&runtimev1.CheckpointContainerResponse{}, nil)

				err := os.WriteFile(fileLoc, []byte("hello"), 0777)
				require.NoError(t, err)

				args.mockImgSvc.EXPECT().
					Push(mocky.Anything, mocky.Anything, args.baseRef.Context().Tag("checkpoint").String()).
					Return(nil)
			},
		},
		{
			name: "container ready timeout exceeded",
			cfg: Config{
				CPUPeriod:                100,
				CPUQuota:                 200,
				MemoryLimitBytes:         300,
				CheckpointFileDir:        t.TempDir(),
				CheckpointTimeoutSeconds: 60,
				ContainerReadyTimeout:    1 * time.Second,
				RegistryUser:             "user",
				RegistryPass:             "pass",
			},
			state: status.CheckpointStateContainerWaitReadyFailed,
			err:   context.DeadlineExceeded,
			prep: func(args prepArgs) {
				prepUntilContainerAttach(
					args.svc,
					args.checkID,
					args.podID,
					args.ctrID,
					args.mockCRISvc,
					args.baseRef,
					cri.RegistryAuth{
						Username: args.cfg.RegistryUser,
						Password: args.cfg.RegistryPass,
					},
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx     = context.Background()
				logger  = slog.New(slog.NewTextHandler(os.Stdout, nil))
				podID   = "pod-id"
				ctrID   = "container-id"
				checkID = "checkpoint-id"

				mockImgSvc  = mock.NewMockImageService(t)
				mockCRISvc  = mock.NewMockCriService(t)
				statusStore = status.NewMemStore()
				mockExecer  = func(url string) (remotecommand.Executor, error) {
					return &test.RemoteCmdExecutor{}, nil
				}

				svc = NewService(
					logger,
					tt.cfg,
					mockCRISvc,
					mockImgSvc,
					statusStore,
					mockExecer,
					workload.NewPortAllocator(1, 1),
				)
			)

			baseRef, err := name.ParseReference("example.com/test-img:latest")
			require.NoError(t, err)

			tt.prep(prepArgs{
				svc:        svc,
				mockCRISvc: mockCRISvc,
				mockImgSvc: mockImgSvc,
				cfg:        tt.cfg,
				checkID:    checkID,
				podID:      podID,
				ctrID:      ctrID,
				baseRef:    baseRef,
			})

			err = svc.checkpoint(ctx, checkID, baseRef)

			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
			} else {
				require.NoError(t, err)
			}

			st := statusStore.Get(checkID)
			require.Equal(t, tt.state, st.CheckpointStatus.State)
		})
	}
}

func prepUntilContainerAttach(
	svc *ServiceImpl,
	checkID string,
	podID string,
	ctrID string,
	mockCRISvc *mock.MockCriService,
	baseRef name.Reference,
	auth cri.RegistryAuth,
) {
	mockCRISvc.EXPECT().
		EnsureImage(mocky.Anything, baseRef.String(), auth).
		Return(true, nil)

	mockCRISvc.EXPECT().
		RunPodSandbox(mocky.Anything, &runtimev1.RunPodSandboxRequest{
			Config: svc.podConfig(checkID),
		}).
		Return(&runtimev1.RunPodSandboxResponse{
			PodSandboxId: podID,
		}, nil)

	podCfg := svc.podConfig(checkID)

	mockCRISvc.EXPECT().
		RunContainer(mocky.Anything, &runtimev1.CreateContainerRequest{
			PodSandboxId:  podID,
			Config:        svc.ctrConfig(checkID, baseRef.String()),
			SandboxConfig: podCfg,
		}).
		Return(ctrID, nil)

	mockCRISvc.EXPECT().
		Attach(mocky.Anything, &runtimev1.AttachRequest{
			ContainerId: ctrID,
			Stdout:      true,
		}).
		Return(&runtimev1.AttachResponse{}, nil)
}
