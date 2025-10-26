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
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	"github.com/spacechunks/explorer/internal/file"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func ChunkToTransport(domain Chunk) *chunkv1alpha1.Chunk {
	c := &chunkv1alpha1.Chunk{
		Id:          domain.ID,
		Name:        domain.Name,
		Description: domain.Description,
		Tags:        domain.Tags,
		CreatedAt:   timestamppb.New(domain.CreatedAt),
		UpdatedAt:   timestamppb.New(domain.UpdatedAt),
	}

	flavors := make([]*chunkv1alpha1.Flavor, 0, len(domain.Flavors))
	for _, f := range domain.Flavors {
		versions := make([]*chunkv1alpha1.FlavorVersion, 0, len(f.Versions))
		for _, v := range f.Versions {
			versions = append(versions, FlavorVersionToTransport(v))
		}

		flavors = append(flavors, &chunkv1alpha1.Flavor{
			Id:       f.ID,
			Name:     f.Name,
			Versions: versions,
		})
	}

	c.Flavors = flavors

	return c
}

func FlavorToTransport(domain Flavor) *chunkv1alpha1.Flavor {
	return &chunkv1alpha1.Flavor{
		Id:        domain.ID,
		Name:      domain.Name,
		CreatedAt: timestamppb.New(domain.CreatedAt),
		UpdatedAt: timestamppb.New(domain.UpdatedAt),
	}
}

func FlavorToDomain(transport *chunkv1alpha1.Flavor) Flavor {
	return Flavor{
		ID:        transport.GetId(),
		Name:      transport.GetName(),
		CreatedAt: transport.GetCreatedAt().AsTime(),
		UpdatedAt: transport.GetUpdatedAt().AsTime(),
	}
}

func FlavorVersionToDomain(transport *chunkv1alpha1.FlavorVersion) FlavorVersion {
	return FlavorVersion{
		ID:          transport.GetId(),
		Version:     transport.GetVersion(),
		Hash:        transport.GetHash(),
		BuildStatus: BuildStatus(transport.BuildStatus),
		FileHashes:  FileHashSliceToDomain(transport.FileHashes),
		CreatedAt:   transport.GetCreatedAt().AsTime(),
	}
}

func FlavorVersionToTransport(domain FlavorVersion) *chunkv1alpha1.FlavorVersion {
	return &chunkv1alpha1.FlavorVersion{
		Id:            domain.ID,
		Version:       domain.Version,
		Hash:          domain.Hash,
		FileHashes:    FileHashSliceToTransport(domain.FileHashes),
		BuildStatus:   chunkv1alpha1.BuildStatus(chunkv1alpha1.BuildStatus_value[string(domain.BuildStatus)]),
		FilesUploaded: domain.FilesUploaded,
		CreatedAt:     timestamppb.New(domain.CreatedAt),
	}
}

func FileHashSliceToTransport(domain []file.Hash) []*chunkv1alpha1.FileHashes {
	hashes := make([]*chunkv1alpha1.FileHashes, 0, len(domain))
	for _, fh := range domain {
		hashes = append(hashes, &chunkv1alpha1.FileHashes{
			Path: fh.Path,
			Hash: fh.Hash,
		})
	}
	return hashes
}

func FileHashSliceToDomain(transport []*chunkv1alpha1.FileHashes) []file.Hash {
	hashes := make([]file.Hash, 0, len(transport))
	for _, fh := range transport {
		hashes = append(hashes, file.Hash{
			Path: fh.GetPath(),
			Hash: fh.GetHash(),
		})
	}
	return hashes
}
