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

package resource

import (
	"net/netip"
	"time"

	"github.com/spacechunks/explorer/internal/file"
)

const (
	MaxChunkTags             = 4
	MaxChunkNameChars        = 50
	MaxChunkDescriptionChars = 100
)

type Chunk struct {
	ID          string
	Name        string
	Description string
	Tags        []string
	Flavors     []Flavor
	Owner       User
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type BuildStatus string

const (
	BuildStatusPending               BuildStatus = "PENDING"
	BuildStatusBuildImage            BuildStatus = "IMAGE_BUILD"
	BuildStatusBuildCheckpoint       BuildStatus = "CHECKPOINT_BUILD"
	BuildStatusBuildImageFailed      BuildStatus = "IMAGE_BUILD_FAILED"
	BuildStatusBuildCheckpointFailed BuildStatus = "CHECKPOINT_BUILD_FAILED"
	BuildStatusCompleted             BuildStatus = "COMPLETED"
)

type Flavor struct {
	ID        string
	Name      string
	Versions  []FlavorVersion
	CreatedAt time.Time
	UpdatedAt time.Time
}

type FlavorVersionDiff struct {
	Added   []file.Hash
	Removed []file.Hash
	Changed []file.Hash
}

type FlavorVersion struct {
	ID                     string
	Version                string
	MinecraftVersion       string
	Hash                   string
	ChangeHash             string
	FileHashes             []file.Hash
	FilesUploaded          bool
	BuildStatus            BuildStatus
	CreatedAt              time.Time
	PresignedURLExpiryDate *time.Time
	PresignedURL           *string
}

type User struct {
	ID        string
	Nickname  string
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Instance struct {
	ID            string
	Chunk         Chunk
	FlavorVersion FlavorVersion
	Address       netip.Addr
	State         State
	Port          *uint16
	Owner         User
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type StatusReport struct {
	InstanceID string
	State      State
	Port       uint16
}

type State string

const (
	StatePending   State = "PENDING"
	StateCreating  State = "CREATING"
	StateRunning   State = "RUNNING"
	StateDeleting  State = "DELETING"
	StateDeleted   State = "DELETED"
	CreationFailed State = "CREATION_FAILED"
)
