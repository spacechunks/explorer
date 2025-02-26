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

package chunk

import (
	"net/netip"
	"time"
)

type Flavor struct {
	ID                 string
	Name               string
	BaseImageURL       string
	CheckpointImageURL string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type Chunk struct {
	ID          string
	Name        string
	Description string
	Tags        []string
	Flavors     []Flavor
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Instance struct {
	ID          string
	Chunk       Chunk
	ChunkFlavor Flavor
	Address     netip.Addr
	State       InstanceState
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type InstanceState string

const (
	InstanceStatePending  InstanceState = "PENDING"
	InstanceStateStarting InstanceState = "STARTING"
	InstanceStateRunning  InstanceState = "RUNNING"
	InstanceStateDeleting InstanceState = "DELETING"
	InstanceStateDeleted  InstanceState = "DELETED"
)
