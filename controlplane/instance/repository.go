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
	"context"

	"github.com/spacechunks/explorer/internal/resource"
)

type Repository interface {
	CreateInstance(ctx context.Context, instance resource.Instance) (resource.Instance, error)
	ListInstances(ctx context.Context, pageSize int, afterID *string) ([]resource.Instance, error)
	GetInstanceByID(ctx context.Context, id string) (resource.Instance, error)

	// UpdateInstance updates the address and port of the instance with the given id.
	UpdateInstance(ctx context.Context, id string, address string, port uint16) error

	// ApplyStatusReports updates instances rows that are not in [instance.InstanceStateDeleted] state.
	// all other instances will be removed from the table.
	ApplyStatusReports(ctx context.Context, reports []resource.InstanceStatusReport) error
	CountInstancesByFlavorVersionID(ctx context.Context, flavorVersionID string) (uint, error)
}
