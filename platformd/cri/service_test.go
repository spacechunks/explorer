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

package cri_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/spacechunks/explorer/internal/mock"
	"github.com/spacechunks/explorer/platformd/cri"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func TestEnsureImage(t *testing.T) {
	tests := []struct {
		name string
		url  string
		prep func(*mock.MockV1ImageServiceClient, string)
	}{
		{
			name: "pull image if not present",
			prep: func(imgMock *mock.MockV1ImageServiceClient, url string) {
				imgMock.EXPECT().
					ListImages(mocky.Anything, &runtimev1.ListImagesRequest{}).
					Return(&runtimev1.ListImagesResponse{}, nil)

				imgMock.EXPECT().
					PullImage(mocky.Anything, &runtimev1.PullImageRequest{
						Image: &runtimev1.ImageSpec{
							Image: url,
						},
					}).
					Return(&runtimev1.PullImageResponse{}, nil)
			},
		},
		{
			name: "image already present",
			prep: func(imgMock *mock.MockV1ImageServiceClient, url string) {
				imgMock.EXPECT().
					ListImages(mocky.Anything, &runtimev1.ListImagesRequest{}).
					Return(&runtimev1.ListImagesResponse{
						Images: []*runtimev1.Image{
							{
								RepoTags: []string{url},
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
				svc           = cri.NewService(logger, mockRtClient, mockImgClient)
			)

			tt.prep(mockImgClient, tt.url)

			_, err := svc.EnsureImage(ctx, tt.url)
			require.NoError(t, err)
		})
	}
}

func TestEnsurePod(t *testing.T) {
	tests := []struct {
		name string
		opts cri.RunOptions
		prep func(*mock.MockV1RuntimeServiceClient, *mock.MockV1ImageServiceClient, cri.RunOptions)
	}{
		{
			name: "pod is not present",
			opts: cri.RunOptions{
				PodConfig: &runtimev1.PodSandboxConfig{
					Metadata: &runtimev1.PodSandboxMetadata{
						Name:      "pod",
						Uid:       "uid",
						Namespace: "test",
					},
				},
				ContainerConfig: &runtimev1.ContainerConfig{
					Image: &runtimev1.ImageSpec{
						Image:              "image",
						UserSpecifiedImage: "image",
					},
				},
			},
			prep: func(
				rtClient *mock.MockV1RuntimeServiceClient,
				imgClient *mock.MockV1ImageServiceClient,
				opts cri.RunOptions,
			) {
				var (
					sboxID = "abc"
					ctrID  = "def"
				)

				rtClient.EXPECT().
					ListPodSandbox(mocky.Anything, &runtimev1.ListPodSandboxRequest{
						Filter: &runtimev1.PodSandboxFilter{
							LabelSelector: map[string]string{
								cri.LabelPodUID: opts.PodConfig.Metadata.Uid,
							},
						},
					}).
					Return(&runtimev1.ListPodSandboxResponse{}, nil)

				rtClient.EXPECT().
					RunPodSandbox(mocky.Anything, &runtimev1.RunPodSandboxRequest{
						Config: opts.PodConfig,
					}).
					Return(&runtimev1.RunPodSandboxResponse{
						PodSandboxId: sboxID,
					}, nil)

				imgClient.EXPECT().
					ListImages(mocky.Anything, &runtimev1.ListImagesRequest{}).
					Return(&runtimev1.ListImagesResponse{
						Images: []*runtimev1.Image{
							{
								RepoTags: []string{opts.ContainerConfig.Image.Image},
							},
						},
					}, nil)

				opts.ContainerConfig.Metadata = &runtimev1.ContainerMetadata{
					Name: opts.PodConfig.Metadata.Name,
				}

				opts.ContainerConfig.LogPath = fmt.Sprintf(
					"%s_%s",
					opts.PodConfig.Metadata.Namespace,
					opts.PodConfig.Metadata.Name,
				)

				rtClient.EXPECT().
					CreateContainer(mocky.Anything, &runtimev1.CreateContainerRequest{
						PodSandboxId:  sboxID,
						Config:        opts.ContainerConfig,
						SandboxConfig: opts.PodConfig,
					}).
					Return(&runtimev1.CreateContainerResponse{
						ContainerId: ctrID,
					}, nil)

				rtClient.EXPECT().
					StartContainer(mocky.Anything, &runtimev1.StartContainerRequest{
						ContainerId: ctrID,
					}).
					Return(&runtimev1.StartContainerResponse{}, nil)
			},
		},
		{
			name: "pod already present",
			opts: cri.RunOptions{
				PodConfig: &runtimev1.PodSandboxConfig{
					Metadata: &runtimev1.PodSandboxMetadata{
						Uid: "uid",
					},
				},
			},
			prep: func(
				rtClient *mock.MockV1RuntimeServiceClient,
				_ *mock.MockV1ImageServiceClient,
				opts cri.RunOptions,
			) {
				rtClient.EXPECT().
					ListPodSandbox(mocky.Anything, &runtimev1.ListPodSandboxRequest{
						Filter: &runtimev1.PodSandboxFilter{
							LabelSelector: map[string]string{
								cri.LabelPodUID: opts.PodConfig.Metadata.Uid,
							},
						},
					}).
					Return(&runtimev1.ListPodSandboxResponse{
						Items: []*runtimev1.PodSandbox{
							{},
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
				svc           = cri.NewService(logger, mockRtClient, mockImgClient)
			)

			tt.prep(mockRtClient, mockImgClient, tt.opts)

			require.NoError(t, svc.EnsurePod(ctx, tt.opts))
		})
	}
}
