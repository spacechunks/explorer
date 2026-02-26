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
	ErrInvalidBaseImage       = errors.New("invalid base image")
	ErrInvalidOCIRegistry     = errors.New("invalid registry")
)

type CreateImage struct {
	FlavorVersionID string `json:"flavorVersionId"`
	BaseImage       string `json:"baseImage"`
	OCIRegistry     string `json:"registry"`
}

func (CreateImage) Kind() string {
	return "create_image"
}

func (c CreateImage) Validate() error {
	if _, err := uuid.Parse(c.FlavorVersionID); err != nil {
		return ErrInvalidFlavorVersionID
	}

	if c.BaseImage == "" {
		return ErrInvalidBaseImage
	}

	if c.OCIRegistry == "" {
		return ErrInvalidOCIRegistry
	}

	return nil
}

type CreateCheckpoint struct {
	FlavorVersionID string `json:"flavorVersionId"`
	BaseImageURL    string `json:"baseImageUrl"`
}

func (c CreateCheckpoint) Validate() error {
	if _, err := uuid.Parse(c.FlavorVersionID); err != nil {
		return ErrInvalidFlavorVersionID
	}

	if c.BaseImageURL == "" {
		return ErrInvalidBaseImage
	}

	return nil
}

func (CreateCheckpoint) Kind() string {
	return "create_checkpoint"
}

type CreateResourcePack struct {
}

func (CreateResourcePack) Kind() string {
	return "build_resource_pack"
}