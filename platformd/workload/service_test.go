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

package workload_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/spacechunks/explorer/internal/mock"
	"github.com/spacechunks/explorer/platformd/cri"
	"github.com/spacechunks/explorer/platformd/status"
	"github.com/spacechunks/explorer/platformd/workload"
	"github.com/spacechunks/explorer/test"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func TestRunWorkload(t *testing.T) {
	var regAuth = cri.RegistryAuth{
		Username: "user",
		Password: "pass",
	}
	tests := []struct {
		name    string
		w       workload.Workload
		attempt uint
		prep    func(*mock.MockCriService, workload.Workload, uint)
	}{
		{
			name: "everyhing works",
			w: workload.Workload{
				ID:               test.NewUUIDv7(t),
				Name:             "test",
				CheckpointImage:  "test-image",
				Namespace:        "test",
				Hostname:         "test",
				Labels:           map[string]string{"k": "v"},
				CPUPeriod:        100000,
				CPUQuota:         200000,
				MemoryLimitBytes: 100000,
			},
			attempt: 1,
			prep: func(criService *mock.MockCriService, w workload.Workload, attempt uint) {
				var (
					podID   = "pod-test"
					sboxCfg = &runtimev1.PodSandboxConfig{
						Metadata: &runtimev1.PodSandboxMetadata{
							Name:      w.Name,
							Uid:       w.ID,
							Namespace: w.Namespace,
							Attempt:   uint32(attempt),
						},
						Hostname:     w.Hostname,
						LogDirectory: cri.PodLogDir,
						Labels:       w.Labels,
						DnsConfig: &runtimev1.DNSConfig{
							Servers:  []string{"10.0.0.53"},
							Options:  []string{"edns0", "trust-ad"},
							Searches: []string{"."},
						},
						Linux: &runtimev1.LinuxPodSandboxConfig{
							Resources: &runtimev1.LinuxContainerResources{
								CpuPeriod:          int64(w.CPUPeriod),
								CpuQuota:           int64(w.CPUQuota),
								MemoryLimitInBytes: int64(w.MemoryLimitBytes),
							},
						},
					}
					ctrReq = &runtimev1.CreateContainerRequest{
						PodSandboxId: podID,
						Config: &runtimev1.ContainerConfig{
							Metadata: &runtimev1.ContainerMetadata{
								Name: w.Name,
							},
							Image: &runtimev1.ImageSpec{
								UserSpecifiedImage: w.CheckpointImage,
								Image:              w.CheckpointImage,
							},
							Labels:  w.Labels,
							LogPath: fmt.Sprintf("%s_%s", w.Namespace, w.Name),
						},
						SandboxConfig: sboxCfg,
					}
				)

				criService.EXPECT().
					EnsureImage(mocky.Anything, w.BaseImage, regAuth).
					Return(false, nil)

				criService.EXPECT().
					RunPodSandbox(mocky.Anything, &runtimev1.RunPodSandboxRequest{
						Config: sboxCfg,
					}).
					Return(&runtimev1.RunPodSandboxResponse{
						PodSandboxId: podID,
					}, nil)

				criService.EXPECT().
					EnsureImage(mocky.Anything, w.CheckpointImage, regAuth).
					Return(false, nil)

				criService.EXPECT().
					RunContainer(mocky.Anything, ctrReq).
					Return("", nil)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx            = context.Background()
				logger         = slog.New(slog.NewTextHandler(os.Stdout, nil))
				mockCRIService = mock.NewMockCriService(t)
				svc            = workload.NewService(logger, mockCRIService, regAuth)
			)

			tt.prep(mockCRIService, tt.w, tt.attempt)

			err := svc.RunWorkload(ctx, tt.w, tt.attempt)
			require.NoError(t, err)
		})
	}
}

func TestRemoveWorkload(t *testing.T) {
	var (
		ctx     = context.Background()
		wlID    = test.NewUUIDv7(t)
		logger  = slog.New(slog.NewTextHandler(os.Stdout, nil))
		regAuth = cri.RegistryAuth{
			Username: "user",
			Password: "pass",
		}
		mockCRIService = mock.NewMockCriService(t)
		svc            = workload.NewService(logger, mockCRIService, regAuth)
	)

	mockCRIService.EXPECT().
		StopPodSandbox(ctx, &runtimev1.StopPodSandboxRequest{
			PodSandboxId: wlID,
		}).
		Return(&runtimev1.StopPodSandboxResponse{}, nil)

	mockCRIService.EXPECT().
		RemovePodSandbox(ctx, &runtimev1.StopPodSandboxRequest{
			PodSandboxId: wlID,
		}).
		Return(&runtimev1.RemovePodSandboxResponse{}, nil)

	require.NoError(t, svc.RemoveWorkload(ctx, wlID))
}

func TestGetWorkloadHealth(t *testing.T) {
	tests := []struct {
		name     string
		state    runtimev1.ContainerState
		expected status.WorkloadHealthStatus
	}{
		{
			name:     "HEALTHY: ContainerState_CONTAINER_RUNNING",
			state:    runtimev1.ContainerState_CONTAINER_RUNNING,
			expected: status.WorkloadHealthStatusHealthy,
		},
		{
			name:     "UNHEALTHY: ContainerState_CONTAINER_CREATED",
			state:    runtimev1.ContainerState_CONTAINER_CREATED,
			expected: status.WorkloadHealthStatusUnhealthy,
		},
		{
			name:     "UNHEALTHY: ContainerState_CONTAINER_UNKNOWN",
			state:    runtimev1.ContainerState_CONTAINER_UNKNOWN,
			expected: status.WorkloadHealthStatusUnhealthy,
		},
		{
			name:     "UNHEALTHY: ContainerState_CONTAINER_EXITED",
			state:    runtimev1.ContainerState_CONTAINER_EXITED,
			expected: status.WorkloadHealthStatusUnhealthy,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx            = context.Background()
				logger         = slog.New(slog.NewTextHandler(os.Stdout, nil))
				mockCRIService = mock.NewMockCriService(t)
				regAuth        = cri.RegistryAuth{
					Username: "user",
					Password: "pass",
				}
				svc = workload.NewService(logger, mockCRIService, regAuth)
			)

			mockCRIService.EXPECT().
				ListContainers(ctx, &runtimev1.ListContainersRequest{
					Filter: &runtimev1.ContainerFilter{
						LabelSelector: map[string]string{
							workload.LabelWorkloadID: "",
						},
					},
				}).
				Return(&runtimev1.ListContainersResponse{
					Containers: []*runtimev1.Container{
						{
							State: tt.state,
						},
					},
				}, nil)

			st, err := svc.GetWorkloadHealth(ctx, "")
			require.NoError(t, err)

			require.Equal(t, tt.expected, st)
		})
	}
}
