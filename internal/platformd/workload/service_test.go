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

	"github.com/spacechunks/platform/internal/mock"
	"github.com/spacechunks/platform/internal/platformd/workload"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

const testWorkloadID = "29533179-f25a-49e8-b2f7-ffb187327692"

func TestEnsureWorkload(t *testing.T) {
	tests := []struct {
		name          string
		labelSelector map[string]string
		opts          workload.RunOptions
		prep          func(
			*mock.MockV1RuntimeServiceClient,
			*mock.MockV1ImageServiceClient,
			workload.RunOptions,
			map[string]string,
		)
	}{
		{
			name:          "everything works",
			labelSelector: map[string]string{"k": "v"},
			opts: workload.RunOptions{
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
				Args:      []string{"--test"},
				DNSServer: "127.0.0.1",
			},
			prep: func(
				rtMock *mock.MockV1RuntimeServiceClient,
				imgMock *mock.MockV1ImageServiceClient,
				opts workload.RunOptions,
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
			opts:          workload.RunOptions{},
			prep: func(
				rtMock *mock.MockV1RuntimeServiceClient,
				imgMock *mock.MockV1ImageServiceClient,
				opts workload.RunOptions,
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
			ctx := context.Background()
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			mockRtClient := mock.NewMockV1RuntimeServiceClient(t)
			mockImgClient := mock.NewMockV1ImageServiceClient(t)
			svc := workload.NewService(logger, mockRtClient, mockImgClient)

			os.Setenv("TEST_WORKLOAD_ID", testWorkloadID)

			tt.prep(mockRtClient, mockImgClient, tt.opts, tt.labelSelector)

			require.NoError(t, svc.EnsureWorkload(ctx, tt.opts, tt.labelSelector))
		})
	}
}

func TestRunWorkload(t *testing.T) {
	var (
		opts = workload.RunOptions{
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
			Args:      []string{"--test"},
			DNSServer: "127.0.0.1",
		}
		wlID = "29533179-f25a-49e8-b2f7-ffb187327692"
	)
	tests := []struct {
		name string
		opts workload.RunOptions
		prep func(
			*mock.MockV1RuntimeServiceClient,
			*mock.MockV1ImageServiceClient,
			workload.RunOptions,
		)
	}{
		{
			name: "all options set - pull image if not present",
			opts: opts,
			prep: func(rtMock *mock.MockV1RuntimeServiceClient,
				imgMock *mock.MockV1ImageServiceClient,
				opts workload.RunOptions,
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
			opts: opts,
			prep: func(rtMock *mock.MockV1RuntimeServiceClient,
				imgMock *mock.MockV1ImageServiceClient,
				opts workload.RunOptions,
			) {
				imgMock.EXPECT().
					ListImages(mocky.Anything, &runtimev1.ListImagesRequest{}).
					Return(&runtimev1.ListImagesResponse{
						Images: []*runtimev1.Image{
							{
								RepoTags: []string{opts.Image},
							},
						},
					}, nil)

				expect(rtMock, opts, wlID)
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

			os.Setenv("TEST_WORKLOAD_ID", wlID)

			tt.prep(mockRtClient, mockImgClient, tt.opts)

			wl, err := svc.RunWorkload(ctx, tt.opts)
			require.NoError(t, err)

			expected := workload.Workload{
				ID:                   wlID,
				Name:                 tt.opts.Name,
				Image:                tt.opts.Image,
				Namespace:            tt.opts.Namespace,
				Hostname:             tt.opts.Hostname,
				Labels:               tt.opts.Labels,
				NetworkNamespaceMode: tt.opts.NetworkNamespaceMode,
			}

			require.Equal(t, expected, wl)
		})
	}
}

// expect runs all expectations required for a successful pod creation and container start
func expect(rtMock *mock.MockV1RuntimeServiceClient, opts workload.RunOptions, wlID string) {
	var (
		ctrID   = "ctr-test"
		podID   = "pod-test"
		sboxCfg = &runtimev1.PodSandboxConfig{
			Metadata: &runtimev1.PodSandboxMetadata{
				Name:      opts.Name,
				Uid:       wlID,
				Namespace: opts.Namespace,
			},
			Hostname:     opts.Hostname,
			LogDirectory: workload.PodLogDir,
			Labels:       opts.Labels,
			DnsConfig: &runtimev1.DNSConfig{
				Servers:  []string{opts.DNSServer},
				Options:  []string{"edns0", "trust-ad"},
				Searches: []string{"."},
			},
			Linux: &runtimev1.LinuxPodSandboxConfig{
				SecurityContext: &runtimev1.LinuxSandboxSecurityContext{
					NamespaceOptions: &runtimev1.NamespaceOption{
						Network: runtimev1.NamespaceMode(opts.NetworkNamespaceMode),
					},
				},
			},
		}
		ctrReq = &runtimev1.CreateContainerRequest{
			PodSandboxId: podID,
			Config: &runtimev1.ContainerConfig{
				Metadata: &runtimev1.ContainerMetadata{
					Name:    opts.Name,
					Attempt: 0,
				},
				Image: &runtimev1.ImageSpec{
					UserSpecifiedImage: opts.Image,
					Image:              opts.Image,
				},
				Labels:  opts.Labels,
				LogPath: fmt.Sprintf("%s_%s", opts.Namespace, opts.Name),
				Args:    opts.Args,
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

	mnts := make([]*runtimev1.Mount, 0, len(opts.Mounts))
	for _, m := range opts.Mounts {
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
