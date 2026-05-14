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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	"github.com/spacechunks/explorer/internal/mock"
	"github.com/spacechunks/explorer/internal/resource"
	"github.com/spacechunks/explorer/internal/resource/codec"
	"github.com/spacechunks/explorer/platformd/cri"
	"github.com/spacechunks/explorer/platformd/status"
	"github.com/spacechunks/explorer/platformd/workload"
	"github.com/spacechunks/explorer/test"
	"github.com/spacechunks/explorer/test/fixture"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
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
		cfg     workload.Config
		attempt uint
		prep    func(*mock.MockCriService, workload.Config, workload.Workload, uint)
	}{
		{
			name: "everyhing works",
			w: workload.Workload{
				ID:               test.NewUUIDv7(t),
				CheckpointImage:  "test-image",
				Name:             "test",
				Namespace:        "test",
				Hostname:         "test",
				Labels:           map[string]string{"k": "v"},
				CPUPeriod:        100000,
				CPUQuota:         200000,
				MemoryLimitBytes: 100000,
				Instance:         codec.InstanceToTransport(fixture.Instance()),
			},
			cfg: workload.Config{
				MCManagementAPIToken:   "some-token",
				ServerMonImage:         "server-mon",
				PlatformdListenSockURL: test.MustParseURL(t, "unix:///var/run/platform.sock"),
				PlatformdSocketUID:     1337,
				PlatformdSocketGID:     1337,
			},
			attempt: 1,
			prep: func(criService *mock.MockCriService, cfg workload.Config, w workload.Workload, attempt uint) {
				data, err := protojson.Marshal(w.Instance)
				require.NoError(t, err)

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
						LogDirectory: podLogDir(w.Instance),
						Labels:       w.Labels,
						DnsConfig: &runtimev1.DNSConfig{
							Servers:  []string{"10.0.0.53"},
							Options:  []string{"edns0", "trust-ad"},
							Searches: []string{"."},
						},
						Annotations: map[string]string{
							workload.AnnotationInstance: string(data),
						},
						Linux: &runtimev1.LinuxPodSandboxConfig{
							Resources: &runtimev1.LinuxContainerResources{
								CpuPeriod:          int64(w.CPUPeriod),
								CpuQuota:           int64(w.CPUQuota),
								MemoryLimitInBytes: int64(w.MemoryLimitBytes),
							},
							Sysctls: map[string]string{
								"net.ipv4.ip_unprivileged_port_start": "0",
							},
						},
					}
					mcCtrReq = &runtimev1.CreateContainerRequest{
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
							LogPath: "mcserver.log",
						},
						SandboxConfig: sboxCfg,
					}
					serverMonCtrReq = &runtimev1.CreateContainerRequest{
						PodSandboxId: podID,
						Config: &runtimev1.ContainerConfig{
							Metadata: &runtimev1.ContainerMetadata{
								Name: "servermon",
							},
							Labels: map[string]string{
								workload.LabelWorkloadID: w.ID,
							},
							Image: &runtimev1.ImageSpec{
								UserSpecifiedImage: cfg.ServerMonImage,
								Image:              cfg.ServerMonImage,
							},
							LogPath: "servermon.log",
							Mounts: []*runtimev1.Mount{
								{
									HostPath:      cfg.PlatformdListenSockURL.Path,
									ContainerPath: cfg.PlatformdListenSockURL.Path,
								},
							},
							Linux: &runtimev1.LinuxContainerConfig{
								SecurityContext: &runtimev1.LinuxContainerSecurityContext{
									RunAsUser: &runtimev1.Int64Value{
										Value: int64(cfg.PlatformdSocketUID),
									},
									RunAsGroup: &runtimev1.Int64Value{
										Value: int64(cfg.PlatformdSocketGID),
									},
								},
							},
							Envs: []*runtimev1.KeyValue{
								{
									Key:   "PLATFORMD_WORKLOAD_ID",
									Value: w.ID,
								},
								{
									Key:   "SERVERMON_MC_SERVER_MANAGEMENT_API_TOKEN",
									Value: cfg.MCManagementAPIToken,
								},
								{
									Key:   "SERVERMON_PLATFORMD_LISTEN_SOCK",
									Value: cfg.PlatformdListenSockURL.String(),
								},
							},
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
					RunContainer(mocky.Anything, mcCtrReq).
					Return("", nil)

				criService.EXPECT().
					EnsureImage(mocky.Anything, cfg.ServerMonImage, cri.Unauthenticated).
					Return(false, nil)

				criService.EXPECT().
					RunContainer(mocky.Anything, serverMonCtrReq).
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
				svc            = workload.NewService(logger, tt.cfg, mockCRIService, regAuth)
			)

			tt.prep(mockCRIService, tt.cfg, tt.w, tt.attempt)

			err := svc.RunWorkload(ctx, tt.w, tt.attempt)
			require.NoError(t, err)
		})
	}
}

func TestRemoveWorkload(t *testing.T) {
	var (
		ctx     = context.Background()
		wlID    = test.NewUUIDv7(t)
		podID   = "pod-test"
		logger  = slog.New(slog.NewTextHandler(os.Stdout, nil))
		regAuth = cri.RegistryAuth{
			Username: "user",
			Password: "pass",
		}
		mockCRIService = mock.NewMockCriService(t)
		svc            = workload.NewService(logger, workload.Config{}, mockCRIService, regAuth)
	)

	mockCRIService.EXPECT().
		ListPodSandbox(ctx, &runtimev1.ListPodSandboxRequest{
			Filter: &runtimev1.PodSandboxFilter{
				LabelSelector: map[string]string{
					workload.LabelWorkloadID: wlID,
				},
			},
		}).
		Return(&runtimev1.ListPodSandboxResponse{
			Items: []*runtimev1.PodSandbox{
				{
					Id: podID,
				},
			},
		}, nil)

	mockCRIService.EXPECT().
		StopPodSandbox(ctx, &runtimev1.StopPodSandboxRequest{
			PodSandboxId: podID,
		}).
		Return(&runtimev1.StopPodSandboxResponse{}, nil)

	mockCRIService.EXPECT().
		RemovePodSandbox(ctx, &runtimev1.RemovePodSandboxRequest{
			PodSandboxId: podID,
		}).
		Return(&runtimev1.RemovePodSandboxResponse{}, nil)

	mockCRIService.EXPECT().
		ListContainers(ctx, &runtimev1.ListContainersRequest{
			Filter: &runtimev1.ContainerFilter{
				PodSandboxId: podID,
			},
		}).
		Return(&runtimev1.ListContainersResponse{
			Containers: []*runtimev1.Container{
				{
					Id: "container-1",
				},
				{
					Id: "container-2",
				},
			},
		}, nil)

	mockCRIService.EXPECT().
		RemoveContainer(ctx, &runtimev1.RemoveContainerRequest{
			ContainerId: "container-1",
		}).
		Return(&runtimev1.RemoveContainerResponse{}, nil)

	mockCRIService.EXPECT().
		RemoveContainer(ctx, &runtimev1.RemoveContainerRequest{
			ContainerId: "container-2",
		}).
		Return(&runtimev1.RemoveContainerResponse{}, nil)

	require.NoError(t, svc.RemoveWorkload(ctx, wlID))
}

func TestGetWorkloadHealth(t *testing.T) {
	tests := []struct {
		name     string
		states   []runtimev1.ContainerState
		expected status.WorkloadHealthStatus
	}{
		{
			name: "HEALTHY: all ContainerState_CONTAINER_RUNNING",
			states: []runtimev1.ContainerState{
				runtimev1.ContainerState_CONTAINER_RUNNING,
				runtimev1.ContainerState_CONTAINER_RUNNING,
			},
			expected: status.WorkloadHealthStatusHealthy,
		},
		{
			name: "UNHEALTHY: ContainerState_CONTAINER_CREATED",
			states: []runtimev1.ContainerState{
				runtimev1.ContainerState_CONTAINER_CREATED,
				runtimev1.ContainerState_CONTAINER_RUNNING,
			},
			expected: status.WorkloadHealthStatusUnhealthy,
		},
		{
			name: "UNHEALTHY: ContainerState_CONTAINER_UNKNOWN",
			states: []runtimev1.ContainerState{
				runtimev1.ContainerState_CONTAINER_UNKNOWN,
				runtimev1.ContainerState_CONTAINER_RUNNING,
			},
			expected: status.WorkloadHealthStatusUnhealthy,
		},
		{
			name: "UNHEALTHY: ContainerState_CONTAINER_EXITED",
			states: []runtimev1.ContainerState{
				runtimev1.ContainerState_CONTAINER_EXITED,
				runtimev1.ContainerState_CONTAINER_RUNNING,
			},
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
				svc = workload.NewService(logger, workload.Config{}, mockCRIService, regAuth)
			)

			var ctrs []*runtimev1.Container
			for _, s := range tt.states {
				ctrs = append(ctrs, &runtimev1.Container{
					Metadata: &runtimev1.ContainerMetadata{
						Name: "ctr",
					},
					State: s,
				})
			}

			mockCRIService.EXPECT().
				ListContainers(ctx, &runtimev1.ListContainersRequest{
					Filter: &runtimev1.ContainerFilter{
						LabelSelector: map[string]string{
							workload.LabelWorkloadID: "",
						},
					},
				}).
				Return(&runtimev1.ListContainersResponse{
					Containers: ctrs,
				}, nil)

			st, err := svc.GetWorkloadHealth(ctx, "")
			require.NoError(t, err)

			require.Equal(t, tt.expected, st)
		})
	}
}

func TestWorkloadMetadata(t *testing.T) {
	tests := []struct {
		name     string
		pods     func(*testing.T) []*runtimev1.PodSandbox
		err      error
		expected workload.Metadata
	}{
		{
			name: "works fine",
			pods: func(t *testing.T) []*runtimev1.PodSandbox {
				data, err := protojson.Marshal(codec.InstanceToTransport(fixture.Instance()))
				require.NoError(t, err)

				return []*runtimev1.PodSandbox{
					{
						Id: "blabla",
						Annotations: map[string]string{
							workload.AnnotationInstance: string(data),
						},
					},
				}
			},
			expected: workload.Metadata{
				ID: fixture.Instance().ID,
				Chunk: resource.Chunk{
					ID:          fixture.Instance().Chunk.ID,
					Name:        fixture.Instance().Chunk.Name,
					Description: fixture.Instance().Chunk.Description,
					Tags:        fixture.Instance().Chunk.Tags,
					CreatedAt:   fixture.Instance().CreatedAt,
					UpdatedAt:   fixture.Instance().CreatedAt,
				},
				// whatever
				FlavorVersion: codec.FlavorVersionToDomain(
					codec.FlavorVersionToTransport(fixture.Instance().FlavorVersion),
				),
				OrderedBy: fixture.Instance().OrderedBy,
			},
		},
		{
			name: "error when annotation emtpy",
			pods: func(t *testing.T) []*runtimev1.PodSandbox {
				return []*runtimev1.PodSandbox{
					{
						Id: "blabla",
					},
				}
			},
			err: grpcstatus.Error(codes.Internal, "invalid instance data"),
		},
		{
			name: "error when pods empty",
			pods: func(t *testing.T) []*runtimev1.PodSandbox {
				return nil
			},
			err: grpcstatus.Error(codes.NotFound, "workload not found"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx     = context.Background()
				wlID    = test.NewUUIDv7(t)
				logger  = slog.New(slog.NewTextHandler(os.Stdout, nil))
				regAuth = cri.RegistryAuth{
					Username: "user",
					Password: "pass",
				}
				mockCRIService = mock.NewMockCriService(t)
				svc            = workload.NewService(logger, workload.Config{}, mockCRIService, regAuth)
			)

			mockCRIService.EXPECT().
				ListPodSandbox(ctx, &runtimev1.ListPodSandboxRequest{
					Filter: &runtimev1.PodSandboxFilter{
						LabelSelector: map[string]string{
							workload.LabelWorkloadID: wlID,
						},
					},
				}).
				Return(&runtimev1.ListPodSandboxResponse{
					Items: tt.pods(t),
				}, nil)

			meta, err := svc.WorkloadMetadata(ctx, wlID)

			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)

			if d := cmp.Diff(tt.expected, meta); d != "" {
				t.Fatalf("workload metadata diff (-want, +got): %s", d)
			}
		})
	}
}

func podLogDir(ins *instancev1alpha1.Instance) string {
	var (
		cName = strings.ReplaceAll(ins.Chunk.Name, " ", "-")
		fName = strings.ReplaceAll(ins.Flavor.Name, " ", "-")
	)
	return fmt.Sprintf("%s/%s_%s_%s_%s_%s_%s_%s",
		cri.PodLogDir,
		cName,
		fName,
		ins.FlavorVersion.Version,
		ins.Chunk.Id,
		ins.FlavorVersion.Id,
		ins.Id,
		ins.Owner.Id,
	)
}
