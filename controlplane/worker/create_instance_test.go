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
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/spacechunks/explorer/controlplane/serverapi"
	"github.com/spacechunks/explorer/controlplane/worker"
	"github.com/spacechunks/explorer/internal/mock"
	"github.com/spacechunks/explorer/internal/resource"
	"github.com/spacechunks/explorer/test/fixture"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const registryEndpoint = "registry.example.com"

func TestCreateInstanceWorker(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		state   serverapi.State
		err     error
		attempt int
		// 0 means no attempts have been made yet, which is the
		// correct default for most test cases.
		maxAttempts int
	}{
		{
			name:    "works",
			timeout: 10 * time.Second,
			state:   serverapi.StateRunning,
		},
		{
			name:        "job timeout exceeded",
			timeout:     30 * time.Millisecond,
			state:       serverapi.StateCreating,
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
				logger           = slog.New(slog.NewTextHandler(os.Stdout, nil))
				mockInsRepo      = mock.NewMockInstanceRepository(t)
				mockServerClient = mock.NewMockServerapiClient(t)
				ins              = fixture.Instance()
				serverID         = "server-id"
				port             = uint16(25565)
				image            = fmt.Sprintf("%s/%s:base", registryEndpoint, ins.FlavorVersion.ID)
			)

			mockInsRepo.EXPECT().
				GetInstanceByID(mocky.Anything, ins.ID).
				Return(ins, nil)

			mockServerClient.EXPECT().
				CreateServer(mocky.Anything, ins.ID, image).
				Return(serverID, nil)

			mockServerClient.EXPECT().
				GetServer(mocky.Anything, serverID).
				Return(serverapi.Status{
					State: tt.state,
					Network: &serverapi.Network{
						Address: "10.0.0.1",
						Port:    port,
					},
				}, nil)

			if tt.state == serverapi.StateRunning {
				mockInsRepo.EXPECT().
					ApplyStatusReports(mocky.Anything, []resource.InstanceStatusReport{
						{
							InstanceID: ins.ID,
							State:      resource.InstanceStateRunning,
							Port:       port,
						},
					}).
					Return(nil)
			}

			w := worker.NewCreateInstanceWorker(
				logger,
				mockInsRepo,
				mockServerClient,
				worker.CreateInstanceWorkerConfig{
					Timeout:             tt.timeout,
					StatusCheckInterval: 5 * time.Millisecond,
					RegistryEndpoint:    registryEndpoint,
				},
			)

			riverJob := &river.Job[job.CreateInstance]{
				JobRow: &rivertype.JobRow{
					Attempt:     tt.attempt,
					MaxAttempts: tt.maxAttempts,
				},
				Args: job.CreateInstance{
					InstanceID: ins.ID,
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

func TestCreateInstanceWorkerServerError(t *testing.T) {
	var (
		ctx              = context.Background()
		logger           = slog.New(slog.NewTextHandler(os.Stdout, nil))
		mockInsRepo      = mock.NewMockInstanceRepository(t)
		mockServerClient = mock.NewMockServerapiClient(t)
		ins              = fixture.Instance()
		serverID         = "server-id"
		image            = fmt.Sprintf("%s/%s:base", registryEndpoint, ins.FlavorVersion.ID)
	)

	mockInsRepo.EXPECT().
		GetInstanceByID(mocky.Anything, ins.ID).
		Return(ins, nil)

	mockServerClient.EXPECT().
		CreateServer(mocky.Anything, ins.ID, image).
		Return(serverID, nil)

	mockServerClient.EXPECT().
		GetServer(mocky.Anything, serverID).
		Return(serverapi.Status{
			State: serverapi.StateError,
		}, nil)

	mockServerClient.EXPECT().
		DeleteServer(mocky.Anything, serverID).
		Return(nil)

	mockInsRepo.EXPECT().
		ApplyStatusReports(mocky.Anything, []resource.InstanceStatusReport{
			{
				InstanceID: ins.ID,
				State:      resource.InstanceStateDeleted,
			},
		}).
		Return(nil)

	w := worker.NewCreateInstanceWorker(
		logger,
		mockInsRepo,
		mockServerClient,
		worker.CreateInstanceWorkerConfig{
			Timeout:             10 * time.Second,
			StatusCheckInterval: 5 * time.Millisecond,
			RegistryEndpoint:    registryEndpoint,
		},
	)

	riverJob := &river.Job[job.CreateInstance]{
		JobRow: &rivertype.JobRow{},
		Args: job.CreateInstance{
			InstanceID: ins.ID,
		},
	}

	err := w.Work(ctx, riverJob)

	var cancelErr *river.JobCancelError
	require.ErrorAs(t, err, &cancelErr)
	require.ErrorContains(t, err, serverID)
}

func TestCreateInstanceWorkerDeleteServerFailsStillCancelsJob(t *testing.T) {
	var (
		ctx              = context.Background()
		logger           = slog.New(slog.NewTextHandler(os.Stdout, nil))
		mockInsRepo      = mock.NewMockInstanceRepository(t)
		mockServerClient = mock.NewMockServerapiClient(t)
		ins              = fixture.Instance()
		serverID         = "server-id"
		image            = fmt.Sprintf("%s/%s:base", registryEndpoint, ins.FlavorVersion.ID)
	)

	mockInsRepo.EXPECT().
		GetInstanceByID(mocky.Anything, ins.ID).
		Return(ins, nil)

	mockServerClient.EXPECT().
		CreateServer(mocky.Anything, ins.ID, image).
		Return(serverID, nil)

	mockServerClient.EXPECT().
		GetServer(mocky.Anything, serverID).
		Return(serverapi.Status{
			State: serverapi.StateError,
		}, nil)

	mockServerClient.EXPECT().
		DeleteServer(mocky.Anything, serverID).
		Return(errors.New("delete failed"))

	mockInsRepo.EXPECT().
		ApplyStatusReports(mocky.Anything, []resource.InstanceStatusReport{
			{
				InstanceID: ins.ID,
				State:      resource.InstanceStateDeleted,
			},
		}).
		Return(nil)

	w := worker.NewCreateInstanceWorker(
		logger,
		mockInsRepo,
		mockServerClient,
		worker.CreateInstanceWorkerConfig{
			Timeout:             10 * time.Second,
			StatusCheckInterval: 5 * time.Millisecond,
			RegistryEndpoint:    registryEndpoint,
		},
	)

	riverJob := &river.Job[job.CreateInstance]{
		JobRow: &rivertype.JobRow{},
		Args: job.CreateInstance{
			InstanceID: ins.ID,
		},
	}

	err := w.Work(ctx, riverJob)

	var cancelErr *river.JobCancelError
	require.ErrorAs(t, err, &cancelErr)
}

func TestCreateInstanceWorkerCreateServerFails(t *testing.T) {
	var (
		ctx              = context.Background()
		logger           = slog.New(slog.NewTextHandler(os.Stdout, nil))
		mockInsRepo      = mock.NewMockInstanceRepository(t)
		mockServerClient = mock.NewMockServerapiClient(t)
		ins              = fixture.Instance()
		image            = fmt.Sprintf("%s/%s:base", registryEndpoint, ins.FlavorVersion.ID)
		wantErr          = errors.New("create server failed")
	)

	mockInsRepo.EXPECT().
		GetInstanceByID(mocky.Anything, ins.ID).
		Return(ins, nil)

	mockServerClient.EXPECT().
		CreateServer(mocky.Anything, ins.ID, image).
		Return("", wantErr)

	w := worker.NewCreateInstanceWorker(
		logger,
		mockInsRepo,
		mockServerClient,
		worker.CreateInstanceWorkerConfig{
			Timeout:             10 * time.Second,
			StatusCheckInterval: 5 * time.Millisecond,
			RegistryEndpoint:    registryEndpoint,
		},
	)

	riverJob := &river.Job[job.CreateInstance]{
		JobRow: &rivertype.JobRow{},
		Args: job.CreateInstance{
			InstanceID: ins.ID,
		},
	}

	err := w.Work(ctx, riverJob)
	require.ErrorIs(t, err, wantErr)
}

func TestCreateInstanceWorkerInstanceNotFound(t *testing.T) {
	var (
		ctx              = context.Background()
		logger           = slog.New(slog.NewTextHandler(os.Stdout, nil))
		mockInsRepo      = mock.NewMockInstanceRepository(t)
		mockServerClient = mock.NewMockServerapiClient(t)
		ins              = fixture.Instance()
		wantErr          = errors.New("instance not found")
	)

	mockInsRepo.EXPECT().
		GetInstanceByID(mocky.Anything, ins.ID).
		Return(resource.Instance{}, wantErr)

	w := worker.NewCreateInstanceWorker(
		logger,
		mockInsRepo,
		mockServerClient,
		worker.CreateInstanceWorkerConfig{
			Timeout:             10 * time.Second,
			StatusCheckInterval: 5 * time.Millisecond,
			RegistryEndpoint:    registryEndpoint,
		},
	)

	riverJob := &river.Job[job.CreateInstance]{
		JobRow: &rivertype.JobRow{},
		Args: job.CreateInstance{
			InstanceID: ins.ID,
		},
	}

	err := w.Work(ctx, riverJob)
	require.ErrorIs(t, err, wantErr)
}

func TestCreateInstanceWorkerInvalidArgs(t *testing.T) {
	var (
		ctx              = context.Background()
		logger           = slog.New(slog.NewTextHandler(os.Stdout, nil))
		mockInsRepo      = mock.NewMockInstanceRepository(t)
		mockServerClient = mock.NewMockServerapiClient(t)
	)

	w := worker.NewCreateInstanceWorker(
		logger,
		mockInsRepo,
		mockServerClient,
		worker.CreateInstanceWorkerConfig{
			Timeout:             10 * time.Second,
			StatusCheckInterval: 5 * time.Millisecond,
			RegistryEndpoint:    registryEndpoint,
		},
	)

	riverJob := &river.Job[job.CreateInstance]{
		JobRow: &rivertype.JobRow{},
		Args: job.CreateInstance{
			InstanceID: "not-a-uuid",
		},
	}

	err := w.Work(ctx, riverJob)
	require.ErrorIs(t, err, job.ErrInvalidInstanceID)
}
