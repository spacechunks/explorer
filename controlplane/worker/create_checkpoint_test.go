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

package worker_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	checkpointv1alpha1 "github.com/spacechunks/explorer/api/platformd/checkpoint/v1alpha1"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/spacechunks/explorer/controlplane/node"
	"github.com/spacechunks/explorer/controlplane/resource"
	"github.com/spacechunks/explorer/controlplane/worker"
	"github.com/spacechunks/explorer/internal/mock"
	"github.com/spacechunks/explorer/test"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCreateCheckpointWorker(t *testing.T) {
	tests := []struct {
		name        string
		timeout     time.Duration
		state       checkpointv1alpha1.CheckpointState
		buildStatus resource.FlavorVersionBuildStatus
		err         error
		attempt     int
		maxAttempts int
	}{
		{
			name:        "works",
			timeout:     10 * time.Second,
			state:       checkpointv1alpha1.CheckpointState_COMPLETED,
			buildStatus: resource.FlavorVersionBuildStatusCompleted,
		},
		{
			name:        "job timeout exceeded",
			timeout:     30 * time.Millisecond,
			state:       checkpointv1alpha1.CheckpointState_RUNNING,
			buildStatus: resource.FlavorVersionBuildStatusBuildCheckpointFailed,
			err:         context.DeadlineExceeded,
			attempt:     1,
			maxAttempts: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			var (
				logger        = slog.New(slog.NewTextHandler(os.Stdout, nil))
				mockNodeRepo  = mock.NewMockNodeRepository(t)
				mockChunkRepo = mock.NewMockChunkRepository(t)
				mockClient    = mock.NewMockV1alpha1CheckpointServiceClient(t)
				newClient     = func(_ string) (checkpointv1alpha1.CheckpointServiceClient, error) {
					return mockClient, nil
				}
				baseImgURL      = "some-url"
				checkID         = "checkpoint-id"
				flavorVersionID = test.NewUUIDv7(t)
			)

			mockNodeRepo.EXPECT().
				RandomNode(mocky.Anything).
				Return(node.Node{}, nil) // return value doesn't matter

			mockClient.EXPECT().
				CreateCheckpoint(mocky.Anything, &checkpointv1alpha1.CreateCheckpointRequest{
					BaseImageUrl: baseImgURL,
				}).
				Return(&checkpointv1alpha1.CreateCheckpointResponse{
					CheckpointId: checkID,
				}, nil)

			mockClient.EXPECT().
				CheckpointStatus(mocky.Anything, &checkpointv1alpha1.CheckpointStatusRequest{
					CheckpointId: checkID,
				}).
				Return(&checkpointv1alpha1.CheckpointStatusResponse{
					Status: &checkpointv1alpha1.CheckpointStatus{
						State: tt.state,
					},
				}, nil)

			mockChunkRepo.EXPECT().
				UpdateFlavorVersionBuildStatus(mocky.Anything, flavorVersionID, tt.buildStatus).
				Return(nil)

			w := worker.NewCheckpointWorker(
				logger,
				newClient,
				mockNodeRepo,
				mockChunkRepo,
				worker.CreateCheckpointWorkerConfig{
					Timeout:             tt.timeout,
					StatusCheckInterval: 5 * time.Millisecond,
				},
			)

			riverJob := &river.Job[job.CreateCheckpoint]{
				JobRow: &rivertype.JobRow{
					Attempt:     tt.attempt,
					MaxAttempts: tt.maxAttempts,
				},
				Args: job.CreateCheckpoint{
					FlavorVersionID: flavorVersionID,
					BaseImageURL:    baseImgURL,
				},
			}

			err := w.Work(ctx, riverJob)

			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}

			require.NoError(t, err)
		})
	}
}
