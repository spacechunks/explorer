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
	"github.com/spacechunks/explorer/platformd/workload"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

const testWorkloadID = "29533179-f25a-49e8-b2f7-ffb187327692"

func TestEnsureWorkload(t *testing.T) {
	tests := []struct {
		name          string
		labelSelector map[string]string
		opts          workload.Workload
		prep          func(
			*mock.MockV1RuntimeServiceClient,
			*mock.MockV1ImageServiceClient,
			workload.Workload,
			map[string]string,
		)
	}{
		{
			name:          "everything works",
			labelSelector: map[string]string{"k": "v"},
			opts: workload.Workload{
				ID:                   testWorkloadID,
				Name:                 "test",
				Image:                "test-image",
				Namespace:            "test",
				Hostname:             "test",
				Labels:               map[string]string{"k": "v"},
				NetworkNamespaceMode: 1,
				Mounts: []workload.Mount{
					{
						ContainerPath: "/etc/test",
						HostPath:      "/tmp/test",
					},
				},
				Args: []string{"--test"},
			},
			prep: func(
				rtMock *mock.MockV1RuntimeServiceClient,
				imgMock *mock.MockV1ImageServiceClient,
				opts workload.Workload,
				labelSelector map[string]string,
			) {
				rtMock.EXPECT().
					ListPodSandbox(mocky.Anything, &runtimev1.ListPodSandboxRequest{
						Filter: &runtimev1.PodSandboxFilter{
							LabelSelector: labelSelector,
						},
					}).
					Return(&runtimev1.ListPodSandboxResponse{}, nil)

				imgMock.EXPECT().
					ListImages(mocky.Anything, &runtimev1.ListImagesRequest{}).
					Return(&runtimev1.ListImagesResponse{}, nil)

				imgMock.EXPECT().
					PullImage(mocky.Anything, &runtimev1.PullImageRequest{
						Image: &runtimev1.ImageSpec{
							Image: opts.Image,
						},
					}).
					Return(&runtimev1.PullImageResponse{}, nil)

				expect(rtMock, opts, testWorkloadID)
			},
		},
		{
			name:          "return when pod is already present",
			labelSelector: map[string]string{"k": "v"},
			opts:          workload.Workload{},
			prep: func(
				rtMock *mock.MockV1RuntimeServiceClient,
				imgMock *mock.MockV1ImageServiceClient,
				opts workload.Workload,
				labelSelector map[string]string,
			) {
				rtMock.EXPECT().
					ListPodSandbox(mocky.Anything, &runtimev1.ListPodSandboxRequest{
						Filter: &runtimev1.PodSandboxFilter{
							LabelSelector: labelSelector,
						},
					}).
					Return(&runtimev1.ListPodSandboxResponse{
						Items: []*runtimev1.PodSandbox{
							{
								Metadata: &runtimev1.PodSandboxMetadata{
									Name: opts.Name,
								},
							},
						},
					}, nil)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx           = context.Background()
				logger        = slog.New(slog.NewTextHandler(os.Stdout, nil))
				mockRtClient  = mock.NewMockV1RuntimeServiceClient(t)
				mockImgClient = mock.NewMockV1ImageServiceClient(t)
				svc           = workload.NewService(logger, mockRtClient, mockImgClient)
			)

			tt.prep(mockRtClient, mockImgClient, tt.opts, tt.labelSelector)

			require.NoError(t, svc.EnsureWorkload(ctx, tt.opts, tt.labelSelector))
		})
	}
}

func TestRunWorkload(t *testing.T) {
	var (
		opts = workload.Workload{
			ID:                   testWorkloadID,
			Name:                 "test",
			Image:                "test-image",
			Namespace:            "test",
			Hostname:             "test",
			Labels:               map[string]string{"k": "v"},
			NetworkNamespaceMode: 1,
			Mounts: []workload.Mount{
				{
					ContainerPath: "/etc/test",
					HostPath:      "/tmp/test",
				},
			},
			Args: []string{"--test"},
		}
		wlID = "29533179-f25a-49e8-b2f7-ffb187327692"
	)
	tests := []struct {
		name string
		w    workload.Workload
		prep func(
			*mock.MockV1RuntimeServiceClient,
			*mock.MockV1ImageServiceClient,
			workload.Workload,
		)
	}{
		{
			name: "all options set - pull image if not present",
			w:    opts,
			prep: func(rtMock *mock.MockV1RuntimeServiceClient,
				imgMock *mock.MockV1ImageServiceClient,
				opts workload.Workload,
			) {
				imgMock.EXPECT().
					ListImages(mocky.Anything, &runtimev1.ListImagesRequest{}).
					Return(&runtimev1.ListImagesResponse{}, nil)

				imgMock.EXPECT().
					PullImage(mocky.Anything, &runtimev1.PullImageRequest{
						Image: &runtimev1.ImageSpec{
							Image: opts.Image,
						},
					}).
					Return(&runtimev1.PullImageResponse{}, nil)

				expect(rtMock, opts, wlID)
			},
		},
		{
			name: "image already present",
			w:    opts,
			prep: func(rtMock *mock.MockV1RuntimeServiceClient,
				imgMock *mock.MockV1ImageServiceClient,
				w workload.Workload,
			) {
				imgMock.EXPECT().
					ListImages(mocky.Anything, &runtimev1.ListImagesRequest{}).
					Return(&runtimev1.ListImagesResponse{
						Images: []*runtimev1.Image{
							{
								RepoTags: []string{w.Image},
							},
						},
					}, nil)

				expect(rtMock, w, wlID)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx           = context.Background()
				logger        = slog.New(slog.NewTextHandler(os.Stdout, nil))
				mockRtClient  = mock.NewMockV1RuntimeServiceClient(t)
				mockImgClient = mock.NewMockV1ImageServiceClient(t)
				svc           = workload.NewService(logger, mockRtClient, mockImgClient)
			)

			tt.prep(mockRtClient, mockImgClient, tt.w)

			err := svc.RunWorkload(ctx, tt.w)
			require.NoError(t, err)
		})
	}
}

func TestRemoveWorkload(t *testing.T) {
	var (
		ctx           = context.Background()
		logger        = slog.New(slog.NewTextHandler(os.Stdout, nil))
		mockRtClient  = mock.NewMockV1RuntimeServiceClient(t)
		mockImgClient = mock.NewMockV1ImageServiceClient(t)
		svc           = workload.NewService(logger, mockRtClient, mockImgClient)
	)

	mockRtClient.EXPECT().StopPodSandbox(ctx, &runtimev1.StopPodSandboxRequest{
		PodSandboxId: testWorkloadID,
	}).Return(&runtimev1.StopPodSandboxResponse{}, nil)

	require.NoError(t, svc.RemoveWorkload(ctx, testWorkloadID))
}

func TestGetWorkload(t *testing.T) {
	tests := []struct {
		name string
		prep func(*mock.MockV1RuntimeServiceClient, workload.Workload)
		err  error
	}{
		{
			name: "everything works",
			prep: func(rtMock *mock.MockV1RuntimeServiceClient, w workload.Workload) {
				rtMock.EXPECT().ListPodSandbox(mocky.Anything, &runtimev1.ListPodSandboxRequest{
					Filter: &runtimev1.PodSandboxFilter{
						LabelSelector: map[string]string{
							workload.LabelWorkloadID: testWorkloadID,
						},
					},
				}).Return(&runtimev1.ListPodSandboxResponse{
					Items: []*runtimev1.PodSandbox{
						{
							Id: testWorkloadID,
							Metadata: &runtimev1.PodSandboxMetadata{
								Name:      w.Name,
								Namespace: w.Namespace,
							},
							Labels: w.Labels,
						},
					},
				}, nil)

				rtMock.EXPECT().ListContainers(mocky.Anything, &runtimev1.ListContainersRequest{
					Filter: &runtimev1.ContainerFilter{
						PodSandboxId: testWorkloadID,
					},
				}).Return(&runtimev1.ListContainersResponse{
					Containers: []*runtimev1.Container{
						{
							Image: &runtimev1.ImageSpec{
								Image: w.Image,
							},
						},
					},
				}, nil)
			},
		},
		{
			name: "pod not found",
			prep: func(rtMock *mock.MockV1RuntimeServiceClient, w workload.Workload) {
				rtMock.EXPECT().ListPodSandbox(mocky.Anything, &runtimev1.ListPodSandboxRequest{
					Filter: &runtimev1.PodSandboxFilter{
						LabelSelector: map[string]string{
							workload.LabelWorkloadID: testWorkloadID,
						},
					},
				}).Return(&runtimev1.ListPodSandboxResponse{}, nil)
			},
			err: workload.ErrWorkloadNotFound,
		},
		{
			name: "container not found",
			prep: func(rtMock *mock.MockV1RuntimeServiceClient, w workload.Workload) {
				rtMock.EXPECT().ListPodSandbox(mocky.Anything, &runtimev1.ListPodSandboxRequest{
					Filter: &runtimev1.PodSandboxFilter{
						LabelSelector: map[string]string{
							workload.LabelWorkloadID: testWorkloadID,
						},
					},
				}).Return(&runtimev1.ListPodSandboxResponse{
					Items: []*runtimev1.PodSandbox{
						{
							Id: testWorkloadID,
							Metadata: &runtimev1.PodSandboxMetadata{
								Name:      w.Name,
								Namespace: w.Namespace,
							},
							Labels: w.Labels,
						},
					},
				}, nil)

				rtMock.EXPECT().ListContainers(mocky.Anything, &runtimev1.ListContainersRequest{
					Filter: &runtimev1.ContainerFilter{
						PodSandboxId: testWorkloadID,
					},
				}).Return(&runtimev1.ListContainersResponse{}, nil)
			},
			err: workload.ErrContainerNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx           = context.Background()
				logger        = slog.New(slog.NewTextHandler(os.Stdout, nil))
				mockRtClient  = mock.NewMockV1RuntimeServiceClient(t)
				mockImgClient = mock.NewMockV1ImageServiceClient(t)
				svc           = workload.NewService(logger, mockRtClient, mockImgClient)
				w             = workload.Workload{
					ID:                   testWorkloadID,
					Status:               workload.Status{},
					Name:                 "test-name",
					Image:                "test-image",
					Namespace:            "test-ns",
					Hostname:             testWorkloadID,
					Labels:               map[string]string{"k": "v"},
					NetworkNamespaceMode: int32(runtimev1.NamespaceMode_NODE),
				}
			)
			tt.prep(mockRtClient, w)

			_, err := svc.GetWorkload(ctx, w.ID)

			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}

			require.NoError(t, err)
		})
	}
}

// expect runs all expectations required for a successful pod creation and container start
func expect(rtMock *mock.MockV1RuntimeServiceClient, w workload.Workload, wlID string) {
	var (
		ctrID   = "ctr-test"
		podID   = "pod-test"
		sboxCfg = &runtimev1.PodSandboxConfig{
			Metadata: &runtimev1.PodSandboxMetadata{
				Name:      w.Name,
				Uid:       wlID,
				Namespace: w.Namespace,
			},
			Hostname:     w.Hostname,
			LogDirectory: workload.PodLogDir,
			Labels:       w.Labels,
			DnsConfig: &runtimev1.DNSConfig{
				Servers:  []string{"10.0.0.53"},
				Options:  []string{"edns0", "trust-ad"},
				Searches: []string{"."},
			},
			Linux: &runtimev1.LinuxPodSandboxConfig{
				SecurityContext: &runtimev1.LinuxSandboxSecurityContext{
					NamespaceOptions: &runtimev1.NamespaceOption{
						Network: runtimev1.NamespaceMode(w.NetworkNamespaceMode),
					},
				},
			},
		}
		ctrReq = &runtimev1.CreateContainerRequest{
			PodSandboxId: podID,
			Config: &runtimev1.ContainerConfig{
				Metadata: &runtimev1.ContainerMetadata{
					Name:    w.Name,
					Attempt: 0,
				},
				Image: &runtimev1.ImageSpec{
					UserSpecifiedImage: w.Image,
					Image:              w.Image,
				},
				Labels:  w.Labels,
				LogPath: fmt.Sprintf("%s_%s", w.Namespace, w.Name),
				Args:    w.Args,
			},
			SandboxConfig: sboxCfg,
		}
	)

	rtMock.EXPECT().
		RunPodSandbox(mocky.Anything, &runtimev1.RunPodSandboxRequest{
			Config: sboxCfg,
		}).
		Return(&runtimev1.RunPodSandboxResponse{
			PodSandboxId: podID,
		}, nil)

	mnts := make([]*runtimev1.Mount, 0, len(w.Mounts))
	for _, m := range w.Mounts {
		mnts = append(mnts, &runtimev1.Mount{
			ContainerPath: m.ContainerPath,
			HostPath:      m.HostPath,
		})
	}

	ctrReq.Config.Mounts = mnts

	rtMock.EXPECT().
		CreateContainer(mocky.Anything, ctrReq).
		Return(&runtimev1.CreateContainerResponse{
			ContainerId: ctrID,
		}, nil)

	rtMock.EXPECT().
		StartContainer(mocky.Anything, &runtimev1.StartContainerRequest{
			ContainerId: ctrID,
		}).
		Return(&runtimev1.StartContainerResponse{}, nil)
}
