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
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	k8stesting "k8s.io/cri-api/pkg/apis/testing"
)

var (
	FakeCRIAddr   = "/run/fakecri/fakecri.sock"
	FakeAttachURL = "http://fake-attach-url"
)

type FakeCRI struct {
	runtimev1.UnimplementedRuntimeServiceServer
	runtimev1.UnimplementedImageServiceServer

	rtSvc  *k8stesting.FakeRuntimeService
	imgSvc *k8stesting.FakeImageService
}

func newFakeCRI() *FakeCRI {
	return &FakeCRI{
		rtSvc:  k8stesting.NewFakeRuntimeService(),
		imgSvc: k8stesting.NewFakeImageService(),
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

func (f *FakeCRI) CheckpointContainer(
	ctx context.Context,
	req *runtimev1.CheckpointContainerRequest,
) (*runtimev1.CheckpointContainerResponse, error) {
	time.Sleep(1 * time.Second) // simulate checkpointing taking some time

	if err := os.WriteFile(req.Location, []byte("checkpoint"), 0777); err != nil {
		return nil, err
	}

	return &runtimev1.CheckpointContainerResponse{}, nil
}

func (f *FakeCRI) Attach(ctx context.Context, req *runtimev1.AttachRequest) (*runtimev1.AttachResponse, error) {
	return &runtimev1.AttachResponse{
		Url: FakeAttachURL,
	}, nil
}

func RunFakeCRI(t *testing.T) {
	criServ := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))

	criSock, err := net.Listen("unix", "@"+FakeCRIAddr)
	require.NoError(t, err)

	cri := newFakeCRI()

	runtimev1.RegisterRuntimeServiceServer(criServ, cri)
	runtimev1.RegisterImageServiceServer(criServ, cri)

	t.Cleanup(func() {
		criServ.Stop()
		criSock.Close()
	})

	go func() {
		err = criServ.Serve(criSock)
		require.NoError(t, err)
	}()
}
