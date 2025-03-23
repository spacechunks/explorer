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

func TestReconciler(t *testing.T) {
	var (
		nodeKey                 = "uggeee"
		namespace               = "test"
		registryEndpoint        = "reg.example.com"
		maxAttempts      uint   = 5
		cpuPeriod        uint64 = 1
		cpuQuota         uint64 = 2
		memoryLimit      uint64 = 3
	)

	expectedWorkload := func(ins *instancev1alpha1.Instance) workload.Workload {
		labels := workload.InstanceLabels(ins)
		labels[workload.LabelWorkloadPort] = "1"

		baseURL := fmt.Sprintf(
			"%s/%s/%s",
			registryEndpoint,
			ins.GetChunk().GetName(),
			ins.GetFlavor().GetName(),
		)

		return workload.Workload{
			ID:               ins.GetId(),
			Name:             ins.GetChunk().GetName() + "_" + ins.GetFlavor().GetName(),
			BaseImage:        baseURL + "/base",
			CheckpointImage:  baseURL + "/checkpoint",
			Namespace:        namespace,
			Hostname:         ins.GetId(),
			Labels:           labels,
			CPUPeriod:        cpuPeriod,
			CPUQuota:         cpuQuota,
			MemoryLimitBytes: memoryLimit,
		}
	}

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
				ins := discoverInstance(t, insClient, nodeKey, instancev1alpha1.InstanceState_PENDING)

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

				store.EXPECT().
					Update(ins.GetId(), workload.Status{
						State: workload.StateCreating,
					})

				store.EXPECT().
					Update(ins.GetId(), workload.Status{
						Port: 1,
					})

				wlSvc.EXPECT().
					RunWorkload(mocky.Anything, expectedWorkload(ins), uint(1)).
					Return(nil)

				store.EXPECT().
					Update(ins.GetId(), workload.Status{
						State: workload.StateRunning,
					})

				store.EXPECT().View().Return(map[string]workload.Status{
					ins.GetId(): {
						State: workload.StateRunning,
						Port:  1,
					},
				})

				expectReportedStatus(insClient, ins.GetId(), instancev1alpha1.InstanceState_RUNNING, 1)
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
				ins := discoverInstance(t, insClient, nodeKey, instancev1alpha1.InstanceState_PENDING)

				store.EXPECT().
					Get(ins.GetId()).
					Return(&workload.Status{
						State: workload.StateCreating,
					})

				attemptCalls := store.EXPECT().
					Update(ins.GetId(), workload.Status{
						State: workload.StateCreating,
					}).
					Times(int(maxAttempts))

				store.EXPECT().
					Update(ins.GetId(), workload.Status{
						State: workload.StateCreationFailed,
					}).
					NotBefore(attemptCalls)

				store.EXPECT().
					Update(ins.GetId(), workload.Status{
						Port: 1,
					}).Times(int(maxAttempts))

				for i := 0; i < int(maxAttempts); i++ {
					wlSvc.EXPECT().
						RunWorkload(mocky.Anything, expectedWorkload(ins), uint(i+1)).
						Return(errors.New("some error"))
				}

				wlSvc.EXPECT().
					RemoveWorkload(mocky.Anything, ins.GetId()).
					Return(nil).
					Times(int(maxAttempts))

				store.EXPECT().View().Return(map[string]workload.Status{
					ins.GetId(): {
						State: workload.StateCreationFailed,
					},
				})

				expectReportedStatus(insClient, ins.GetId(), instancev1alpha1.InstanceState_CREATION_FAILED, 0)

				store.EXPECT().Del(ins.GetId())
			},
		},
		{
			name: "instance DELETING: remove workload",
			prep: func(
				wlSvc *mock.MockWorkloadService,
				insClient *mock.MockV1alpha1InstanceServiceClient,
				store *mock.MockWorkloadStatusStore,
			) {
				ins := discoverInstance(t, insClient, nodeKey, instancev1alpha1.InstanceState_DELETING)

				wlSvc.EXPECT().
					RemoveWorkload(mocky.Anything, ins.GetId()).
					Return(nil)

				store.EXPECT().
					Update(ins.GetId(), workload.Status{
						State: workload.StateDeleted,
					})

				store.EXPECT().View().Return(map[string]workload.Status{
					ins.GetId(): {
						State: workload.StateDeleted,
					},
				})

				expectReportedStatus(insClient, ins.GetId(), instancev1alpha1.InstanceState_DELETED, 0)

				store.EXPECT().Del(ins.GetId())
			},
		},
		{
			name: "instance DELETING: set state to DELETED when instance to remove is not found",
			prep: func(
				wlSvc *mock.MockWorkloadService,
				insClient *mock.MockV1alpha1InstanceServiceClient,
				store *mock.MockWorkloadStatusStore,
			) {
				ins := discoverInstance(t, insClient, nodeKey, instancev1alpha1.InstanceState_DELETING)

				wlSvc.EXPECT().
					RemoveWorkload(mocky.Anything, ins.GetId()).
					Return(status.New(codes.NotFound, "not found").Err())

				store.EXPECT().
					Update(ins.GetId(), workload.Status{
						State: workload.StateDeleted,
					})

				store.EXPECT().View().Return(map[string]workload.Status{
					ins.GetId(): {
						State: workload.StateDeleted,
					},
				})

				expectReportedStatus(insClient, ins.GetId(), instancev1alpha1.InstanceState_DELETED, 0)

				store.EXPECT().Del(ins.GetId())
			},
		},
		{
			name: "instance RUNNING: do nothing if instance is and workload is HEALTHY",
			prep: func(
				wlSvc *mock.MockWorkloadService,
				insClient *mock.MockV1alpha1InstanceServiceClient,
				store *mock.MockWorkloadStatusStore,
			) {
				ins := discoverInstance(t, insClient, nodeKey, instancev1alpha1.InstanceState_RUNNING)
				wlSvc.EXPECT().
					GetWorkloadHealth(mocky.Anything, ins.GetId()).
					Return(workload.HealthStatusHealthy, nil)

				store.EXPECT().View().Return(map[string]workload.Status{})
				insClient.EXPECT().ReceiveInstanceStatusReports(
					mocky.Anything, &instancev1alpha1.ReceiveInstanceStatusReportsRequest{
						Reports: []*instancev1alpha1.InstanceStatusReport{},
					}).
					Return(nil, nil)
			},
		},
		{
			name: "instance RUNNING: remove UNHEALTHY workload",
			prep: func(
				wlSvc *mock.MockWorkloadService,
				insClient *mock.MockV1alpha1InstanceServiceClient,
				store *mock.MockWorkloadStatusStore,
			) {
				ins := discoverInstance(t, insClient, nodeKey, instancev1alpha1.InstanceState_RUNNING)
				wlSvc.EXPECT().
					GetWorkloadHealth(mocky.Anything, ins.GetId()).
					Return(workload.HealthStatusUnhealthy, nil)

				wlSvc.EXPECT().
					RemoveWorkload(mocky.Anything, ins.GetId()).
					Return(nil)

				store.EXPECT().
					Update(ins.GetId(), workload.Status{
						State: workload.StateDeleted,
					})

				store.EXPECT().View().Return(map[string]workload.Status{
					ins.GetId(): {
						State: workload.StateDeleted,
					},
				})

				expectReportedStatus(insClient, ins.GetId(), instancev1alpha1.InstanceState_DELETED, 0)

				store.EXPECT().Del(ins.GetId())
			},
		},
		{
			name: "DELETED and CREATION_FAILED workloads are not deleted from the store when reporting fails",
			prep: func(
				wlSvc *mock.MockWorkloadService,
				insClient *mock.MockV1alpha1InstanceServiceClient,
				store *mock.MockWorkloadStatusStore,
			) {
				ins := discoverInstance(t, insClient, nodeKey, instancev1alpha1.InstanceState_RUNNING)
				wlSvc.EXPECT().
					GetWorkloadHealth(mocky.Anything, ins.GetId()).
					Return(workload.HealthStatusUnhealthy, nil)

				wlSvc.EXPECT().
					RemoveWorkload(mocky.Anything, ins.GetId()).
					Return(nil)

				store.EXPECT().
					Update(ins.GetId(), workload.Status{
						State: workload.StateDeleted,
					})

				store.EXPECT().View().Return(map[string]workload.Status{
					ins.GetId(): {
						State: workload.StateDeleted,
					},
				})

				insClient.EXPECT().ReceiveInstanceStatusReports(
					mocky.Anything, &instancev1alpha1.ReceiveInstanceStatusReportsRequest{
						Reports: []*instancev1alpha1.InstanceStatusReport{
							{
								InstanceId: ptr.Pointer(ins.GetId()),
								State:      ptr.Pointer(instancev1alpha1.InstanceState_DELETED),
								Port:       ptr.Pointer(uint32(0)),
							},
						},
					}).
					Return(nil, errors.New("some error"))
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
				syncer     = newReconciler(
					logger,
					reconcilerConfig{
						MaxAttempts:         maxAttempts,
						SyncInterval:        100 * time.Millisecond,
						NodeID:              nodeKey,
						MinPort:             1,
						MaxPort:             1,
						WorkloadNamespace:   namespace,
						WorkloadCPUPeriod:   cpuPeriod,
						WorkloadCPUQuota:    cpuQuota,
						WorkloadMemoryLimit: memoryLimit,
						RegistryEndpoint:    registryEndpoint,
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

func discoverInstance(
	t *testing.T,
	insClient *mock.MockV1alpha1InstanceServiceClient,
	nodeKey string,
	state instancev1alpha1.InstanceState,
) *instancev1alpha1.Instance {
	ins := &instancev1alpha1.Instance{
		Id: ptr.Pointer(test.NewUUIDv7(t)),
		Chunk: &chunkv1alpha1.Chunk{
			Name: ptr.Pointer("test-chunk"),
		},
		Flavor: &chunkv1alpha1.Flavor{
			Name: ptr.Pointer("test-flavor"),
		},
		State: ptr.Pointer(state),
	}

	insClient.EXPECT().
		DiscoverInstances(mocky.Anything, &instancev1alpha1.DiscoverInstanceRequest{
			NodeKey: &nodeKey,
		}).
		Return(&instancev1alpha1.DiscoverInstanceResponse{
			Instances: []*instancev1alpha1.Instance{
				ins,
			},
		}, nil)

	return ins
}

func expectReportedStatus(
	insClient *mock.MockV1alpha1InstanceServiceClient,
	id string,
	state instancev1alpha1.InstanceState,
	port uint32,
) {
	insClient.EXPECT().ReceiveInstanceStatusReports(
		mocky.Anything, &instancev1alpha1.ReceiveInstanceStatusReportsRequest{
			Reports: []*instancev1alpha1.InstanceStatusReport{
				{
					InstanceId: ptr.Pointer(id),
					State:      ptr.Pointer(state),
					Port:       ptr.Pointer(port),
				},
			},
		}).
		Return(nil, nil)
}
