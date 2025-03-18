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
	"net/netip"
	"time"

	"github.com/spacechunks/explorer/controlplane/chunk"
)

type Instance struct {
	ID          string
	Chunk       chunk.Chunk
	ChunkFlavor chunk.Flavor
	Address     netip.Addr
	State       State
	Port        *uint16
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type StatusReport struct {
	InstanceID string
	State      State
	Port       uint16
}

type State string

const (
	StatePending   State = "PENDING"
	StateRunning   State = "RUNNING"
	StateDeleting  State = "DELETING"
	StateDeleted   State = "DELETED"
	CreationFailed State = "CREATION_FAILED"
)
