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

package publish

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	"github.com/spacechunks/explorer/cli"
	"github.com/spacechunks/explorer/internal/ptr"
)

type buildUpdate struct {
	flavor         localFlavor
	buildStatus    *string
	uploadProgress *uint
	err            error
}

type builder struct {
	client  chunkv1alpha1.ChunkServiceClient
	updates chan buildUpdate
}

func (b builder) build(ctx context.Context, chunkID string, local localFlavor, createRemote bool) {
	// TODO: remove create flavor call, because we can create the flavor in the control plane
	//       when creating the flavor version if needed.

	var flavorID string
	if createRemote {
		resp, err := b.client.CreateFlavor(ctx, &chunkv1alpha1.CreateFlavorRequest{
			ChunkId: chunkID,
			Name:    local.name,
		})
		if err != nil {
			b.updates <- buildUpdate{
				flavor: local,
				err:    fmt.Errorf("error while creating flavor: %w", err),
			}
			return
		}
		flavorID = resp.Flavor.Id
	} else {
		resp, err := b.client.GetChunk(ctx, &chunkv1alpha1.GetChunkRequest{
			Id: chunkID,
		})
		if err != nil {
			b.updates <- buildUpdate{
				flavor: local,
				err:    fmt.Errorf("error while creating flavor: %w", err),
			}
			return
		}
		f := cli.FindFlavor(resp.Chunk.Flavors, func(f *chunkv1alpha1.Flavor) bool {
			return f.Name == local.name
		})
		flavorID = f.Id
	}

	hashes := make([]*chunkv1alpha1.FileHashes, 0, len(local.files))
	for _, fh := range local.files {
		hashes = append(hashes, &chunkv1alpha1.FileHashes{
			Path: local.serverRelPath(fh.Path),
			Hash: fh.Hash,
		})
	}

	versionReq, err := b.client.CreateFlavorVersion(ctx, &chunkv1alpha1.CreateFlavorVersionRequest{
		FlavorId: flavorID,
		Version: &chunkv1alpha1.FlavorVersion{
			Version:    local.version,
			Hash:       local.hash,
			FileHashes: hashes,
		},
	})
	if err != nil {
		b.updates <- buildUpdate{
			flavor: local,
			err:    fmt.Errorf("error while creating flavor version: %w", err),
		}
		return
	}

	files := make([]*chunkv1alpha1.File, 0, len(local.files))
	for _, f := range local.files {
		isAdded := slices.ContainsFunc(versionReq.AddedFiles, func(added *chunkv1alpha1.FileHashes) bool {
			return added.Hash == f.Hash
		})

		isChanged := slices.ContainsFunc(versionReq.ChangedFiles, func(changed *chunkv1alpha1.FileHashes) bool {
			return changed.Hash == f.Hash
		})

		if !isAdded && !isChanged {
			continue
		}

		localPath := filepath.Join(local.path, f.Path)
		data, err := os.ReadFile(localPath)
		if err != nil {
			b.updates <- buildUpdate{
				flavor: local,
				err:    fmt.Errorf("error while reading file %s: %w", localPath, err),
			}
			return
		}
		files = append(files, &chunkv1alpha1.File{
			Path: local.serverRelPath(f.Path),
			Data: data,
		})
	}

	b.updates <- buildUpdate{
		flavor:         local,
		uploadProgress: ptr.Pointer(uint(0)),
	}

	time.Sleep(5 * time.Second)

	if _, err := b.client.SaveFlavorFiles(ctx, &chunkv1alpha1.SaveFlavorFilesRequest{
		FlavorVersionId: versionReq.Version.Id,
		Files:           files,
	}); err != nil {
		b.updates <- buildUpdate{
			flavor: local,
			err:    fmt.Errorf("error while saving flavor files: %w", err),
		}
		return
	}

	if _, err := b.client.BuildFlavorVersion(ctx, &chunkv1alpha1.BuildFlavorVersionRequest{
		FlavorVersionId: versionReq.Version.Id,
	}); err != nil {
		b.updates <- buildUpdate{
			flavor: local,
			err:    fmt.Errorf("error occured when trying to initate server image build: %w", err),
		}
		return
	}

	t := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-t.C:
			c, err := b.client.GetChunk(ctx, &chunkv1alpha1.GetChunkRequest{
				Id: chunkID,
			})
			if err != nil {
				fmt.Println("error while getting chunk:", err)
			}

			flavor := cli.FindFlavor(c.GetChunk().Flavors, func(f *chunkv1alpha1.Flavor) bool {
				return f.Id == flavorID
			})

			b.updates <- buildUpdate{
				flavor:      local,
				buildStatus: ptr.Pointer(flavor.Versions[0].BuildStatus.String()),
			}
		case <-ctx.Done():
			return
		}
	}
}
