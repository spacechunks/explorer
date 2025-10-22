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

package workload

import (
	workloadv1alpha2 "github.com/spacechunks/explorer/api/platformd/workload/v1alpha2"
	"github.com/spacechunks/explorer/platformd/status"
)

func StatusToTransport(st status.Status) *workloadv1alpha2.WorkloadStatus {
	wst := &workloadv1alpha2.WorkloadStatus{}

	if st.WorkloadStatus != nil {
		return &workloadv1alpha2.WorkloadStatus{
			State: StateToTransport(st.WorkloadStatus.State),
			Port:  uint32(st.WorkloadStatus.Port),
		}
	}

	if st.CheckpointStatus != nil {
		return &workloadv1alpha2.WorkloadStatus{
			Port: uint32(st.CheckpointStatus.Port),
		}
	}

	return wst
}

func StateToTransport(state status.WorkloadState) workloadv1alpha2.WorkloadState {
	num, ok := workloadv1alpha2.WorkloadState_value[string(state)]
	if !ok {
		return workloadv1alpha2.WorkloadState_UNKNOWN
	}
	return workloadv1alpha2.WorkloadState(num)
}
