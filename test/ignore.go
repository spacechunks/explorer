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

package test

import (
	"github.com/google/go-cmp/cmp"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	"google.golang.org/protobuf/testing/protocmp"
)

var (
	IgnoredProtoChunkFields = protocmp.IgnoreFields(
		&chunkv1alpha1.Chunk{},
		// created dynamically
		"id",
		"created_at",
		"updated_at",
	)

	IgnoredProtoFlavorVersionFields = protocmp.IgnoreFields(
		&chunkv1alpha1.FlavorVersion{},
		// created dynamically
		"id",
		"created_at",
	)

	IgnoredProtoFlavorFields = protocmp.IgnoreFields(
		&chunkv1alpha1.Flavor{},
		// created dynamically
		"id",
		"created_at",
		"updated_at",
	)

	IgnoredProtoInstanceFields = protocmp.IgnoreFields(
		&instancev1alpha1.Instance{},
		// created dynamically
		"id",
	)

	IgnoredInstanceFields = []string{
		// created dynamically
		"ID",
		"CreatedAt",
		"UpdatedAt",
		"ChunkFlavor.ID",
		"ChunkFlavor.CreatedAt",
		"ChunkFlavor.UpdatedAt",
		"Chunk.Flavors.ID",
		"Chunk.Flavors.CreatedAt",
		"Chunk.Flavors.UpdatedAt",
		"Chunk.ID",
		"Chunk.CreatedAt",
		"Chunk.UpdatedAt",
	}
)

func IgnoreFields(fields ...string) cmp.Option {
	return cmp.FilterPath(func(path cmp.Path) bool {
		for _, f := range fields {
			if f == path.String() {
				return true
			}
		}
		return false
	}, cmp.Ignore())
}
