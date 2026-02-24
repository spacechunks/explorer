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
	MaxChunkTags                = 4
	MaxChunkNameChars           = 50
	MaxChunkDescriptionChars    = 100
	MaxChunkThumbnailDimensions = 512
)

/*
 * chunk-related types
 */

type Chunk struct {
	ID          string
	Name        string
	Description string
	Tags        []string
	Flavors     []Flavor
	Owner       User
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Thumbnail   Thumbnail
}

type Thumbnail struct {
	Hash string
}

/*
 * flavor-related types
 */

type FlavorVersionBuildStatus string

const (
	FlavorVersionBuildStatusPending               FlavorVersionBuildStatus = "PENDING"
	FlavorVersionBuildStatusBuildImage            FlavorVersionBuildStatus = "IMAGE_BUILD"
	FlavorVersionBuildStatusBuildCheckpoint       FlavorVersionBuildStatus = "CHECKPOINT_BUILD"
	FlavorVersionBuildStatusBuildImageFailed      FlavorVersionBuildStatus = "IMAGE_BUILD_FAILED"
	FlavorVersionBuildStatusBuildCheckpointFailed FlavorVersionBuildStatus = "CHECKPOINT_BUILD_FAILED"
	FlavorVersionBuildStatusCompleted             FlavorVersionBuildStatus = "COMPLETED"
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
	BuildStatus            FlavorVersionBuildStatus
	CreatedAt              time.Time
	PresignedURLExpiryDate *time.Time
	PresignedURL           *string
}

/*
 * user-related types
 */

type User struct {
	ID        string
	Nickname  string
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

/*
 * instance-related types
 */

type Instance struct {
	ID            string
	Chunk         Chunk
	FlavorVersion FlavorVersion
	Address       netip.Addr
	State         InstanceState
	Port          *uint16
	Owner         User
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type InstanceStatusReport struct {
	InstanceID string
	State      InstanceState
	Port       uint16
}

type InstanceState string

const (
	InstanceStatePending   InstanceState = "PENDING"
	InstanceStateCreating  InstanceState = "CREATING"
	InstanceStateRunning   InstanceState = "RUNNING"
	InstanceStateDeleting  InstanceState = "DELETING"
	InstanceStateDeleted   InstanceState = "DELETED"
	InstanceCreationFailed InstanceState = "CREATION_FAILED"
)
