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

package instance

import (
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	"github.com/spacechunks/explorer/internal/ptr"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// FromDomain converts the domain object to a transport layer object
func FromDomain(ins Instance) *instancev1alpha1.Instance {
	state := instancev1alpha1.InstanceState(instancev1alpha1.InstanceState_value[string(ins.State)])
	return &instancev1alpha1.Instance{
		Id: &ins.ID,
		Chunk: &chunkv1alpha1.Chunk{
			Id:          &ins.Chunk.ID,
			Name:        &ins.Chunk.Name,
			Description: &ins.Chunk.Description,
			Tags:        ins.Chunk.Tags,
			CreatedAt:   timestamppb.New(ins.Chunk.CreatedAt),
			UpdatedAt:   timestamppb.New(ins.Chunk.UpdatedAt),
		},
		Address: ptr.String(ins.Address.String()),
		State:   &state,
	}
}
