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

package status

import "time"

type Status struct {
	CheckpointStatus *CheckpointStatus
	WorkloadStatus   *WorkloadStatus
}

type WorkloadState string

var (
	WorkloadStateCreating       WorkloadState = "CREATING"
	WorkloadStateRunning        WorkloadState = "RUNNING"
	WorkloadStateDeleted        WorkloadState = "DELETED"
	WorkloadStateCreationFailed WorkloadState = "CREATION_FAILED"
)

type WorkloadHealthStatus string

var (
	WorkloadHealthStatusHealthy   WorkloadHealthStatus = "HEALTHY"
	WorkloadHealthStatusUnhealthy WorkloadHealthStatus = "UNHEALTHY"
)

type WorkloadStatus struct {
	State WorkloadState
	Port  uint16
}

type CheckpointState string

const (
	CheckpointStateRunning                   CheckpointState = "RUNNING"
	CheckpointStatePullBaseImageFailed       CheckpointState = "PULL_BASE_IMAGE_FAILED"
	CheckpointStateContainerWaitReadyFailed  CheckpointState = "CONTAINER_WAIT_READY_FAILED"
	CheckpointStateContainerCheckpointFailed CheckpointState = "CONTAINER_CHECKPOINT_FAILED"
	CheckpointStatePushCheckpointFailed      CheckpointState = "PUSH_CHECKPOINT_FAILED"
	CheckpointStateCompleted                 CheckpointState = "COMPLETED"
)

type CheckpointStatus struct {
	State       CheckpointState
	Message     string
	CompletedAt *time.Time
	Port        uint16
}
