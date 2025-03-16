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
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	"github.com/spacechunks/explorer/internal/mock"
	"github.com/spacechunks/explorer/internal/ptr"
	"github.com/spacechunks/explorer/platformd/workload"
	"github.com/spacechunks/explorer/test"
	mocky "github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSyncer(t *testing.T) {
	var (
		nodeKey          = "uggeee"
		namespace        = "test"
		registryEndpoint = "reg.example.com"
		maxAttempts      = 5
	)

	tests := []struct {
		name string
		prep func(*mock.MockWorkloadService, *mock.MockV1alpha1InstanceServiceClient, *mock.MockWorkloadStatusStore)
	}{
		{
			name: "instance PENDING: create workload",
			prep: func(
				wlSvc *mock.MockWorkloadService,
				insClient *mock.MockV1alpha1InstanceServiceClient,
				store *mock.MockWorkloadStatusStore,
			) {
				ins := instanceFixture(t, instancev1alpha1.InstanceState_PENDING)

				insClient.EXPECT().
					DiscoverInstances(mocky.Anything, &instancev1alpha1.DiscoverInstanceRequest{
						NodeKey: &nodeKey,
					}).
					Return(&instancev1alpha1.DiscoverInstanceResponse{
						Instances: []*instancev1alpha1.Instance{
							ins,
						},
					}, nil)

				// this makes sure that the FIRST call of store.Get does not return anything,
				// but calls after the first Get return that the workload is already running.
				veryFirstGetCall := store.EXPECT().
					Get(ins.GetId()).
					Return(nil).
					Once()
				store.EXPECT().
					Get(ins.GetId()).
					Return(&workload.Status{
						State: workload.StateRunning,
						Port:  1,
					}).
					NotBefore(veryFirstGetCall)

				labels := workload.InstanceLabels(ins)
				labels[workload.LabelWorkloadPort] = "1"

				store.EXPECT().
					Update(ins.GetId(), workload.Status{
						State: workload.StateCreating,
					})

				wlSvc.EXPECT().
					RunWorkload(
						mocky.Anything,
						workload.Workload{
							ID:   ins.GetId(),
							Name: ins.GetChunk().GetName() + "_" + ins.GetFlavor().GetName(),
							Image: fmt.Sprintf(
								"%s/%s/%s",
								registryEndpoint,
								ins.GetChunk().GetName(),
								ins.GetFlavor().GetName(),
							),
							Namespace: namespace,
							Hostname:  ins.GetId(),
							Labels:    labels,
						},
						1,
					).
					Return(nil)

				store.EXPECT().
					Update(ins.GetId(), workload.Status{
						State: workload.StateRunning,
						Port:  1,
					})
			},
		},
		{
			// this test also ensures that the port will be
			// freed when it is supposed to, because if not
			// the port allocation will fail and causes the
			// expected .Times(maxAttempts) on RunWorkload
			// to not be satisfied.
			name: "instance PENDING: max attempts reached",
			prep: func(
				wlSvc *mock.MockWorkloadService,
				insClient *mock.MockV1alpha1InstanceServiceClient,
				store *mock.MockWorkloadStatusStore,
			) {
				ins := instanceFixture(t, instancev1alpha1.InstanceState_PENDING)

				insClient.EXPECT().
					DiscoverInstances(mocky.Anything, &instancev1alpha1.DiscoverInstanceRequest{
						NodeKey: &nodeKey,
					}).
					Return(&instancev1alpha1.DiscoverInstanceResponse{
						Instances: []*instancev1alpha1.Instance{
							ins,
						},
					}, nil)

				store.EXPECT().
					Get(ins.GetId()).
					Return(&workload.Status{
						State: workload.StateCreating,
					})

				attemptCalls := store.EXPECT().
					Update(ins.GetId(), workload.Status{
						State: workload.StateCreating,
					}).
					Times(maxAttempts)

				store.EXPECT().
					Update(ins.GetId(), workload.Status{
						State: workload.StateCreationFailed,
					}).
					NotBefore(attemptCalls)

				labels := workload.InstanceLabels(ins)
				labels[workload.LabelWorkloadPort] = "1"

				for i := 0; i < maxAttempts; i++ {
					wlSvc.EXPECT().
						RunWorkload(
							mocky.Anything,
							workload.Workload{
								ID:   ins.GetId(),
								Name: ins.GetChunk().GetName() + "_" + ins.GetFlavor().GetName(),
								Image: fmt.Sprintf(
									"%s/%s/%s",
									registryEndpoint,
									ins.GetChunk().GetName(),
									ins.GetFlavor().GetName(),
								),
								Namespace: namespace,
								Hostname:  ins.GetId(),
								Labels:    labels,
							},
							i+1,
						).
						Return(errors.New("some error"))
				}

				wlSvc.EXPECT().
					RemoveWorkload(mocky.Anything, ins.GetId()).
					Return(nil).
					Times(maxAttempts)
			},
		},
		{
			name: "instance DELETING: remove workload",
			prep: func(
				wlSvc *mock.MockWorkloadService,
				insClient *mock.MockV1alpha1InstanceServiceClient,
				store *mock.MockWorkloadStatusStore,
			) {
				ins := instanceFixture(t, instancev1alpha1.InstanceState_DELETING)

				insClient.EXPECT().
					DiscoverInstances(mocky.Anything, &instancev1alpha1.DiscoverInstanceRequest{
						NodeKey: &nodeKey,
					}).
					Return(&instancev1alpha1.DiscoverInstanceResponse{
						Instances: []*instancev1alpha1.Instance{
							ins,
						},
					}, nil)

				wlSvc.EXPECT().
					RemoveWorkload(mocky.Anything, ins.GetId()).
					Return(nil)

				store.EXPECT().
					Update(ins.GetId(), workload.Status{
						State: workload.StateDeleted,
					})
			},
		},
		{
			name: "instance DELETING: set state to DELETED when instance to remove is not found",
			prep: func(
				wlSvc *mock.MockWorkloadService,
				insClient *mock.MockV1alpha1InstanceServiceClient,
				store *mock.MockWorkloadStatusStore,
			) {
				ins := instanceFixture(t, instancev1alpha1.InstanceState_DELETING)
				insClient.EXPECT().
					DiscoverInstances(mocky.Anything, &instancev1alpha1.DiscoverInstanceRequest{
						NodeKey: &nodeKey,
					}).
					Return(&instancev1alpha1.DiscoverInstanceResponse{
						Instances: []*instancev1alpha1.Instance{
							ins,
						},
					}, nil)

				wlSvc.EXPECT().
					RemoveWorkload(mocky.Anything, ins.GetId()).
					Return(status.New(codes.NotFound, "not found").Err())

				store.EXPECT().
					Update(ins.GetId(), workload.Status{
						State: workload.StateDeleted,
					})
			},
		},
		{
			name: "instance RUNNING: do nothing if instance is and workload is HEALTHY",
			prep: func(
				wlSvc *mock.MockWorkloadService,
				insClient *mock.MockV1alpha1InstanceServiceClient,
				store *mock.MockWorkloadStatusStore,
			) {
				ins := instanceFixture(t, instancev1alpha1.InstanceState_RUNNING)
				insClient.EXPECT().
					DiscoverInstances(mocky.Anything, &instancev1alpha1.DiscoverInstanceRequest{
						NodeKey: &nodeKey,
					}).
					Return(&instancev1alpha1.DiscoverInstanceResponse{
						Instances: []*instancev1alpha1.Instance{
							ins,
						},
					}, nil)

				wlSvc.EXPECT().
					GetWorkloadHealth(mocky.Anything, ins.GetId()).
					Return(workload.HealthStatusHealthy, nil)
			},
		},
		{
			name: "instance RUNNING: remove unhealthy workload",
			prep: func(
				wlSvc *mock.MockWorkloadService,
				insClient *mock.MockV1alpha1InstanceServiceClient,
				store *mock.MockWorkloadStatusStore,
			) {
				ins := instanceFixture(t, instancev1alpha1.InstanceState_RUNNING)
				insClient.EXPECT().
					DiscoverInstances(mocky.Anything, &instancev1alpha1.DiscoverInstanceRequest{
						NodeKey: &nodeKey,
					}).
					Return(&instancev1alpha1.DiscoverInstanceResponse{
						Instances: []*instancev1alpha1.Instance{
							ins,
						},
					}, nil)

				wlSvc.EXPECT().
					GetWorkloadHealth(mocky.Anything, ins.GetId()).
					Return(workload.HealthStatusHealthy, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx        = context.Background()
				logger     = slog.New(slog.NewTextHandler(os.Stdout, nil))
				mockStore  = mock.NewMockWorkloadStatusStore(t)
				mockWlSvc  = mock.NewMockWorkloadService(t)
				mockInsSvc = mock.NewMockV1alpha1InstanceServiceClient(t)
				syncer     = newSyncer(
					logger,
					syncerConfig{
						MaxAttempts:       maxAttempts,
						SyncInterval:      100 * time.Millisecond,
						NodeID:            nodeKey,
						MinPort:           1,
						MaxPort:           1,
						WorkloadNamespace: namespace,
						RegistryEndpoint:  registryEndpoint,
					},
					mockInsSvc,
					mockWlSvc,
					mockStore,
				)
			)

			tt.prep(mockWlSvc, mockInsSvc, mockStore)

			time.AfterFunc(1*time.Second, func() {
				syncer.Stop()
			})

			syncer.Start(ctx)
		})
	}
}

func instanceFixture(t *testing.T, state instancev1alpha1.InstanceState) *instancev1alpha1.Instance {
	return &instancev1alpha1.Instance{
		Id: ptr.Pointer(test.NewUUIDv7(t)),
		Chunk: &chunkv1alpha1.Chunk{
			Name: ptr.Pointer("test-chunk"),
		},
		Flavor: &chunkv1alpha1.Flavor{
			Name: ptr.Pointer("test-flavor"),
		},
		State: ptr.Pointer(state),
	}
}
