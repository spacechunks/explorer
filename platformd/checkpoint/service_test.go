package checkpoint_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/spacechunks/explorer/internal/mock"
	"github.com/spacechunks/explorer/internal/ptr"
	"github.com/spacechunks/explorer/platformd/checkpoint"
	"github.com/spacechunks/explorer/platformd/workload"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// TODO: at some point write test to ensure state are set correctly.

func TestCollectGarbage(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name               string
		cfg                checkpoint.Config
		storeItems         map[string]checkpoint.Status
		expectedStoreItems map[string]checkpoint.Status
	}{
		{
			name: "one running one completed both still in store",
			cfg: checkpoint.Config{
				CheckpointFileDir:     t.TempDir(),
				StatusRetentionPeriod: 60 * time.Second,
			},
			storeItems: map[string]checkpoint.Status{
				"1": {
					State: checkpoint.StateRunning,
				},
				"2": {
					State:       checkpoint.StateCompleted,
					CompletedAt: ptr.Pointer(now),
				},
			},
			expectedStoreItems: map[string]checkpoint.Status{
				"1": {
					State: checkpoint.StateRunning,
				},
				"2": {
					State:       checkpoint.StateCompleted,
					CompletedAt: ptr.Pointer(now),
				},
			},
		},
		{
			name: "one running one completed, completed removed from store",
			cfg: checkpoint.Config{
				CheckpointFileDir:     t.TempDir(),
				StatusRetentionPeriod: 1 * time.Second,
			},
			storeItems: map[string]checkpoint.Status{
				"1": {
					State: checkpoint.StateRunning,
				},
				"2": {
					State: checkpoint.StateCompleted,
					CompletedAt: ptr.Pointer(
						time.Date(
							now.Year(),
							now.Month(),
							now.Day(),
							now.Hour(),
							now.Minute(),
							now.Second()-1,
							0,
							now.Location(),
						),
					),
				},
			},
			expectedStoreItems: map[string]checkpoint.Status{
				"1": {
					State: checkpoint.StateRunning,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx        = context.Background()
				logger     = slog.New(slog.NewTextHandler(os.Stdout, nil))
				store      = checkpoint.NewStore()
				mockCRISvc = mock.NewMockCriService(t)
				svc        = checkpoint.NewService(
					logger,
					tt.cfg,
					mockCRISvc,
					nil,
					store,
					nil,
					workload.NewStore(),
					workload.NewPortAllocator(1, 1),
				)
			)

			for id, status := range tt.storeItems {
				store.Update(id, status)
			}

			pods := make([]*runtimev1.PodSandbox, 0, len(store.View()))

			for id, status := range store.View() {
				path := tt.cfg.CheckpointFileDir + "/" + id
				err := os.WriteFile(path, []byte{}, 0777)
				require.NoError(t, err)

				pods = append(pods, &runtimev1.PodSandbox{
					Id: id,
					Metadata: &runtimev1.PodSandboxMetadata{
						Uid: id,
					},
				})

				if status.State == checkpoint.StateRunning {
					continue
				}

				mockCRISvc.EXPECT().
					StopPodSandbox(mocky.Anything, &runtimev1.StopPodSandboxRequest{
						PodSandboxId: id,
					}).
					Return(&runtimev1.StopPodSandboxResponse{}, nil)

				mockCRISvc.EXPECT().
					RemovePodSandbox(mocky.Anything, &runtimev1.RemovePodSandboxRequest{
						PodSandboxId: id,
					}).
					Return(&runtimev1.RemovePodSandboxResponse{}, nil)
			}

			mockCRISvc.EXPECT().
				ListPodSandbox(mocky.Anything, &runtimev1.ListPodSandboxRequest{
					Filter: &runtimev1.PodSandboxFilter{
						LabelSelector: map[string]string{
							workload.LabelWorkloadType: "checkpoint",
						},
					},
				}).
				Return(&runtimev1.ListPodSandboxResponse{
					Items: pods,
				}, nil)

			err := svc.CollectGarbage(ctx)
			require.NoError(t, err)

			for id, status := range store.View() {
				_, err := os.Stat(tt.cfg.CheckpointFileDir + "/" + id)
				if status.State == checkpoint.StateRunning {
					require.NoError(t, err)
					continue
				}

				require.ErrorIs(t, err, os.ErrNotExist)
			}

			if d := cmp.Diff(tt.expectedStoreItems, store.View()); d != "" {
				t.Fatalf("mismatch (-want +got):\n%s", d)
			}
		})
	}
}
