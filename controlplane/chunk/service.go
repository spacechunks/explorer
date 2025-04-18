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
	"context"

	"github.com/spacechunks/explorer/controlplane/blob"
)

type Service interface {
	CreateChunk(ctx context.Context, chunk Chunk) (Chunk, error)
	GetChunk(ctx context.Context, id string) (Chunk, error)
	UpdateChunk(ctx context.Context, new Chunk) (Chunk, error)
	CreateFlavor(ctx context.Context, chunkID string, flavor Flavor) (Flavor, error)
	ListFlavors(ctx context.Context, chunkID string) ([]Flavor, error)
	CreateFlavorVersion(ctx context.Context, version FlavorVersion) (FlavorVersion, FlavorVersionDiff, error)
	SaveFlavorFiles(ctx context.Context, versionID string, files []File) error
}

type svc struct {
	repo      Repository
	blobStore blob.Store
}

func NewService(repo Repository, blobStore blob.Store) Service {
	return &svc{
		repo:      repo,
		blobStore: blobStore,
	}
}
