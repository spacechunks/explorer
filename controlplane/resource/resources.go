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
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Tags        []string   `json:"tags"`
	Flavors     []Flavor   `json:"flavors"`
	Owner       User       `json:"owner"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	Thumbnail   Thumbnail  `json:"thumbnail"`
	DeletedAt   *time.Time `json:"deletedAt"`
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
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Versions  []FlavorVersion `json:"versions"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	DeletedAt *time.Time      `json:"deletedAt"`
}

type FlavorVersionDiff struct {
	Added   []file.Hash
	Removed []file.Hash
	Changed []file.Hash
}

type FlavorVersion struct {
	ID                     string                   `json:"id"`
	Version                string                   `json:"version"`
	MinecraftVersion       string                   `json:"minecraftVersion"`
	Hash                   string                   `json:"hash"`
	ChangeHash             string                   `json:"changeHash"`
	FileHashes             []file.Hash              `json:"fileHashes"`
	FilesUploaded          bool                     `json:"filesUploaded"`
	BuildStatus            FlavorVersionBuildStatus `json:"buildStatus"`
	CreatedAt              time.Time                `json:"createdAt"`
	PresignedURLExpiryDate *time.Time               `json:"presignedURLExpiryDate"`
	PresignedURL           *string                  `json:"presignedURL"`
}

/*
 * user-related types
 */

type User struct {
	ID        string    `json:"id"`
	Nickname  string    `json:"nickname"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
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

/*
 * minecraft version
 */

type MinecraftVersion struct {
	Version   string    `json:"version"`
	ImageURL  string    `json:"imageURL"`
	CreatedAt time.Time `json:"createdAt"`
}
