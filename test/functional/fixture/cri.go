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
	"context"

	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/cri-api/pkg/apis/testing"
)

type FakeCRI struct {
	runtimev1.UnimplementedRuntimeServiceServer
	runtimev1.UnimplementedImageServiceServer

	rtSvc  *testing.FakeRuntimeService
	imgSvc *testing.FakeImageService
}

func newFakeCRI() *FakeCRI {
	return &FakeCRI{
		rtSvc:  testing.NewFakeRuntimeService(),
		imgSvc: testing.NewFakeImageService(),
	}
}

func (f *FakeCRI) RunPodSandbox(
	ctx context.Context,
	req *runtimev1.RunPodSandboxRequest,
) (*runtimev1.RunPodSandboxResponse, error) {
	id, _ := f.rtSvc.RunPodSandbox(ctx, req.Config, req.RuntimeHandler)
	return &runtimev1.RunPodSandboxResponse{PodSandboxId: id}, nil
}

func (f *FakeCRI) ListPodSandbox(ctx context.Context,
	req *runtimev1.ListPodSandboxRequest,
) (*runtimev1.ListPodSandboxResponse, error) {
	l, _ := f.rtSvc.ListPodSandbox(ctx, req.Filter)
	return &runtimev1.ListPodSandboxResponse{
		Items: l,
	}, nil
}

func (f *FakeCRI) CreateContainer(ctx context.Context,
	req *runtimev1.CreateContainerRequest,
) (*runtimev1.CreateContainerResponse, error) {
	id, _ := f.rtSvc.CreateContainer(ctx, req.PodSandboxId, req.Config, req.SandboxConfig)
	return &runtimev1.CreateContainerResponse{
		ContainerId: id,
	}, nil
}

func (f *FakeCRI) StartContainer(ctx context.Context,
	req *runtimev1.StartContainerRequest,
) (*runtimev1.StartContainerResponse, error) {
	_ = f.rtSvc.StartContainer(ctx, req.ContainerId)
	return &runtimev1.StartContainerResponse{}, nil
}

func (f *FakeCRI) ListImages(ctx context.Context,
	req *runtimev1.ListImagesRequest,
) (*runtimev1.ListImagesResponse, error) {
	l, _ := f.imgSvc.ListImages(ctx, req.Filter)
	return &runtimev1.ListImagesResponse{
		Images: l,
	}, nil
}

func (f *FakeCRI) PullImage(ctx context.Context,
	req *runtimev1.PullImageRequest,
) (*runtimev1.PullImageResponse, error) {
	ref, _ := f.imgSvc.PullImage(ctx, req.Image, req.Auth, req.SandboxConfig)
	return &runtimev1.PullImageResponse{
		ImageRef: ref,
	}, nil
}
