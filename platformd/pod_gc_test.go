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

package platformd

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/spacechunks/explorer/internal/mock"
	"github.com/spacechunks/explorer/platformd/workload"
	mocky "github.com/stretchr/testify/mock"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func TestPodGC(t *testing.T) {
	var (
		ctx    = context.Background()
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
		client = mock.NewMockV1RuntimeServiceClient(t)
		gc     = newPodGC(logger, client, 100*time.Millisecond, 5)
	)

	pods := []*runtimev1.PodSandbox{
		{
			Id: "1",
			Metadata: &runtimev1.PodSandboxMetadata{
				Name:    "test1",
				Attempt: 5,
			},
		},
		{
			Id: "2",
			Metadata: &runtimev1.PodSandboxMetadata{
				Name:    "test1",
				Attempt: 2,
			},
		},
	}

	client.EXPECT().
		ListPodSandbox(mocky.Anything, &runtimev1.ListPodSandboxRequest{
			Filter: &runtimev1.PodSandboxFilter{
				LabelSelector: map[string]string{
					workload.LabelWorkloadType: "instance",
				},
			},
		}).
		Return(&runtimev1.ListPodSandboxResponse{
			Items: pods,
		}, nil)

	client.EXPECT().
		RemovePodSandbox(mocky.Anything, &runtimev1.RemovePodSandboxRequest{
			PodSandboxId: "1",
		}).
		Return(nil, nil)

	time.AfterFunc(1*time.Second, func() {
		gc.Stop()
	})

	gc.Start(ctx)
}
