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

func TestRunWorkload(t *testing.T) {
	var (
		opts = workload.Workload{
			ID:               testWorkloadID,
			Name:             "test",
			CheckpointImage:  "test-image",
			Namespace:        "test",
			Hostname:         "test",
			Labels:           map[string]string{"k": "v"},
			CPUPeriod:        100000,
			CPUQuota:         200000,
			MemoryLimitBytes: 100000,
		}
		wlID = "29533179-f25a-49e8-b2f7-ffb187327692"
	)
	tests := []struct {
		name    string
		w       workload.Workload
		attempt int
		prep    func(
			*mock.MockV1RuntimeServiceClient,
			*mock.MockV1ImageServiceClient,
			workload.Workload,
			int,
		)
	}{
		{
			name:    "all options set - pull image if not present",
			w:       opts,
			attempt: 2,
			prep: func(
				rtMock *mock.MockV1RuntimeServiceClient,
				imgMock *mock.MockV1ImageServiceClient,
				opts workload.Workload,
				attempt int,
			) {
				imgMock.EXPECT().
					ListImages(mocky.Anything, &runtimev1.ListImagesRequest{}).
					Return(&runtimev1.ListImagesResponse{}, nil).
					Times(2)

				imgMock.EXPECT().
					PullImage(mocky.Anything, &runtimev1.PullImageRequest{
						Image: &runtimev1.ImageSpec{
							Image: opts.BaseImage,
						},
					}).
					Return(&runtimev1.PullImageResponse{}, nil)

				imgMock.EXPECT().
					PullImage(mocky.Anything, &runtimev1.PullImageRequest{
						Image: &runtimev1.ImageSpec{
							Image: opts.CheckpointImage,
						},
					}).
					Return(&runtimev1.PullImageResponse{}, nil)

				expect(rtMock, opts, wlID, attempt)
			},
		},
		{
			name: "image already present",
			w:    opts,
			prep: func(
				rtMock *mock.MockV1RuntimeServiceClient,
				imgMock *mock.MockV1ImageServiceClient,
				w workload.Workload,
				_ int,
			) {
				imgMock.EXPECT().
					ListImages(mocky.Anything, &runtimev1.ListImagesRequest{}).
					Return(&runtimev1.ListImagesResponse{
						Images: []*runtimev1.Image{
							{
								RepoTags: []string{w.BaseImage},
							},
						},
					}, nil).
					Once()

				imgMock.EXPECT().
					ListImages(mocky.Anything, &runtimev1.ListImagesRequest{}).
					Return(&runtimev1.ListImagesResponse{
						Images: []*runtimev1.Image{
							{
								RepoTags: []string{w.CheckpointImage},
							},
						},
					}, nil).
					Once()

				expect(rtMock, w, wlID, 0)
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

			tt.prep(mockRtClient, mockImgClient, tt.w, tt.attempt)

			err := svc.RunWorkload(ctx, tt.w, tt.attempt)
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

// expect runs all expectations required for a successful pod creation and container start
func expect(rtMock *mock.MockV1RuntimeServiceClient, w workload.Workload, wlID string, attempt int) {
	var (
		ctrID   = "ctr-test"
		podID   = "pod-test"
		sboxCfg = &runtimev1.PodSandboxConfig{
			Metadata: &runtimev1.PodSandboxMetadata{
				Name:      w.Name,
				Uid:       wlID,
				Namespace: w.Namespace,
				Attempt:   uint32(attempt),
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

	rtMock.EXPECT().
		RunPodSandbox(mocky.Anything, &runtimev1.RunPodSandboxRequest{
			Config: sboxCfg,
		}).
		Return(&runtimev1.RunPodSandboxResponse{
			PodSandboxId: podID,
		}, nil)

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
