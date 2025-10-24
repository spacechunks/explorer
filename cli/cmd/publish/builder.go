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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"time"

	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	"github.com/spacechunks/explorer/cli"
	"github.com/spacechunks/explorer/internal/ptr"
	"github.com/spacechunks/explorer/internal/tarhelper"
)

type buildUpdate struct {
	flavor         localFlavor
	buildStatus    *string
	uploadProgress *uint
	err            error
}

type builder struct {
	client       chunkv1alpha1.ChunkServiceClient
	updates      chan buildUpdate
	buildCounter *atomic.Int32
	changeSetDir string
}

func (b builder) build(ctx context.Context, chunkID string, local localFlavor, createRemote bool) {
	// TODO: remove create flavor call, because we can create the flavor in the control plane
	//       when creating the flavor version if needed.
	b.buildCounter.Add(1)
	defer b.buildCounter.Add(-1)

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

	files := make([]*os.File, 0, len(local.files))
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
		f, err := os.Open(localPath)
		if err != nil {
			b.updates <- buildUpdate{
				flavor: local,
				err:    fmt.Errorf("error while opening file %s: %w", localPath, err),
			}
			return
		}

		files = append(files, f)
	}

	changeSet := filepath.Join(b.changeSetDir, versionReq.Version.Id+".tar.gz")

	if err := tarhelper.TarFiles(local.path, files, changeSet); err != nil {
		b.updates <- buildUpdate{
			flavor: local,
			err:    fmt.Errorf("error while taring files: %w", err),
		}
		return
	}

	data, err := os.ReadFile(changeSet)
	if err != nil {
		b.updates <- buildUpdate{
			flavor: local,
			err:    fmt.Errorf("error while reading change set: %w", err),
		}
		return
	}

	progReader := &progressReader{
		size:  float64(len(data)),
		inner: bytes.NewReader(data),
	}

	digest := sha256.Sum256(data)

	uploadURLResp, err := b.client.GetUploadURL(ctx, &chunkv1alpha1.GetUploadURLRequest{
		FlavorVersionId: versionReq.Version.Id,
		TarballHash:     base64.StdEncoding.EncodeToString(digest[:]),
	})
	if err != nil {
		b.updates <- buildUpdate{
			flavor: local,
			err:    fmt.Errorf("error while getting upload url: %w", err),
		}
		return
	}

	req, err := http.NewRequest(http.MethodPut, uploadURLResp.Url, progReader)
	if err != nil {
		b.updates <- buildUpdate{
			flavor: local,
			err:    fmt.Errorf("error while creating upload url: %w", err),
		}
		return
	}

	progReader.OnProgress(func(progress uint) {
		b.updates <- buildUpdate{
			flavor:         local,
			uploadProgress: &progress,
		}
	})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		b.updates <- buildUpdate{
			flavor: local,
			err:    fmt.Errorf("error while uploading: %w", err),
		}
		progReader.StopReporting()
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		upd := buildUpdate{
			flavor: local,
			err:    fmt.Errorf("error while uploading: unexpected status code: %d", resp.StatusCode),
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			b.updates <- upd
			return
		}
		fmt.Println(string(body))
		b.updates <- upd
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

			status := flavor.Versions[0].BuildStatus

			b.updates <- buildUpdate{
				flavor:      local,
				buildStatus: ptr.Pointer(status.String()),
			}

			if status == chunkv1alpha1.BuildStatus_COMPLETED ||
				status == chunkv1alpha1.BuildStatus_IMAGE_BUILD_FAILED ||
				status == chunkv1alpha1.BuildStatus_CHECKPOINT_BUILD_FAILED {
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

func (b builder) OnUpdate(ctx context.Context, f func(update buildUpdate)) {
	for {
		select {
		case u := <-b.updates:
			f(u)
			if b.buildCounter.Load() == 0 {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
