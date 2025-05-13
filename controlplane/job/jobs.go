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

package job

import (
	"github.com/google/uuid"
	"github.com/spacechunks/explorer/controlplane/errors"
)

var (
	ErrInvalidFlavorVersionID = errors.New("invalid flavor version id")
	ErrInvalidFlavorName      = errors.New("invalid flavor name")
	ErrInvalidChunkName       = errors.New("invalid chunk name")
	ErrInvalidBaseImage       = errors.New("invalid base image")
	ErrInvalidRegistry        = errors.New("invalid registry")
)

type CreateImage struct {
	FlavorVersionID string `json:"flavorVersionId"`
	FlavorName      string `json:"flavorName"`
	ChunkName       string `json:"chunkName"`
	BaseImage       string `json:"baseImage"`
	Registry        string `json:"registry"`
}

func (CreateImage) Kind() string {
	return "create_image"
}

func (c CreateImage) Validate() error {
	if _, err := uuid.Parse(c.FlavorVersionID); err != nil {
		return ErrInvalidFlavorVersionID
	}

	if c.FlavorName == "" {
		return ErrInvalidFlavorName
	}

	if c.ChunkName == "" {
		return ErrInvalidChunkName
	}

	if c.BaseImage == "" {
		return ErrInvalidBaseImage
	}

	if c.Registry == "" {
		return ErrInvalidRegistry
	}

	return nil
}

type CreateCheckpoint struct {
	BaseImage string `json:"baseImage"`
}

func (CreateCheckpoint) Kind() string {
	return "create_checkpoint"
}
