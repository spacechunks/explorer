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
	"github.com/spacechunks/explorer/internal/file"
	"github.com/spacechunks/explorer/internal/ptr"
	"github.com/spacechunks/explorer/internal/tarhelper"
)

/*
 * WARNING: The code that follows may make you cry:
 *           A Safety Pig has been provided below for your benefit
 *                              _
 *      _._ _..._ .-',     _.._(`))
 *     '-. `     '  /-._.-'    ',/
 *       )         \            '.
 *      / _    _    |             \
 *     |  a    a    /              |
 *      \   .-.                     ;
 *       '-('' ).-'       ,'       ;
 *          '-;           |      .'
 *            \           \    /
 *            | 7  .__  _.-\   \
 *            | |  |  ``/  /`  /
 *           /,_|  |   /,_/   /
 *              /,_/      '`-'
 *
 * This whole file is completely fucked
 */

type buildUpdate struct {
	data           buildData
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

type buildPhase int

const (
	buildPhasePrerequisites buildPhase = iota
	buildPhaseUpload
	buildPhaseTriggerBuild
	buildPhaseBuildComplete
)

type buildData struct {
	chunkID string
	local   localFlavor
	phase   buildPhase
}

func (b builder) build(ctx context.Context, data buildData) {
	b.buildCounter.Add(1)
	defer b.buildCounter.Add(-1)

	for {
		switch data.phase {
		case buildPhasePrerequisites:
			if err := b.handlePrerequisites(ctx, &data); err != nil {
				b.updates <- buildUpdate{
					data: data,
					err:  err,
				}
				return
			}
		case buildPhaseUpload:
			if err := b.handleUpload(ctx, &data); err != nil {
				b.updates <- buildUpdate{
					data: data,
					err:  err,
				}
				return
			}
		case buildPhaseTriggerBuild:
			if err := b.handleTriggerBuild(ctx, &data); err != nil {
				b.updates <- buildUpdate{
					data: data,
					err:  err,
				}
				return
			}
		case buildPhaseBuildComplete:
			break
		}
		if data.phase == buildPhaseBuildComplete {
			break
		}
	}

	t := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-t.C:
			c, err := b.client.GetChunk(ctx, &chunkv1alpha1.GetChunkRequest{
				Id: data.chunkID,
			})
			if err != nil {
				fmt.Println("error while getting chunk:", err)
			}

			flavor := cli.Find(c.GetChunk().Flavors, func(f *chunkv1alpha1.Flavor) bool {
				return f.Name == data.local.name
			})

			status := flavor.Versions[0].BuildStatus

			b.updates <- buildUpdate{
				data:        data,
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

func (b builder) handlePrerequisites(ctx context.Context, data *buildData) error {
	resp, err := b.client.GetChunk(ctx, &chunkv1alpha1.GetChunkRequest{
		Id: data.chunkID,
	})
	if err != nil {
		return fmt.Errorf("get chunk: %w", err)
	}

	remoteFlavor := cli.Find(resp.Chunk.Flavors, func(item *chunkv1alpha1.Flavor) bool {
		return data.local.name == item.Name
	})
	if remoteFlavor == nil {
		resp, err := b.client.CreateFlavor(ctx, &chunkv1alpha1.CreateFlavorRequest{
			ChunkId: data.chunkID,
			Name:    data.local.name,
		})
		if err != nil {
			return fmt.Errorf("error while creating flavor: %w", err)
		}
		remoteFlavor = resp.Flavor
	}

	hashes := make([]*chunkv1alpha1.FileHashes, 0, len(data.local.files))
	for _, fh := range data.local.files {
		hashes = append(hashes, &chunkv1alpha1.FileHashes{
			Path: data.local.serverRelPath(fh.Path),
			Hash: fh.Hash,
		})
	}

	_, err = b.client.CreateFlavorVersion(ctx, &chunkv1alpha1.CreateFlavorVersionRequest{
		FlavorId: remoteFlavor.Id,
		Version: &chunkv1alpha1.FlavorVersion{
			Version:    data.local.version,
			Hash:       data.local.hash,
			FileHashes: hashes,
		},
	})
	if err != nil {
		return fmt.Errorf("error while creating flavor version: %w", err)
	}

	data.phase = buildPhaseUpload
	return nil
}

func (b builder) handleUpload(ctx context.Context, data *buildData) error {
	fmt.Println("DOWNLOAWDOAWDOAWDOAWODAWD")
	resp, err := b.client.GetChunk(ctx, &chunkv1alpha1.GetChunkRequest{
		Id: data.chunkID,
	})
	if err != nil {
		return fmt.Errorf("get chunk: %w", err)
	}

	remoteFlavor := cli.Find(resp.Chunk.Flavors, func(item *chunkv1alpha1.Flavor) bool {
		return data.local.name == item.Name
	})

	if remoteFlavor == nil {
		return fmt.Errorf("flavor %s not found in chunk", data.local.name)
	}

	remoteVersion := cli.Find(remoteFlavor.Versions, func(v *chunkv1alpha1.FlavorVersion) bool {
		return data.local.hash == v.Hash
	})

	if remoteVersion == nil {
		return fmt.Errorf("could not find flavor version with hash %s. reason for this could be that local files have changed since creating the flavor version", data.local.hash) // nolint:lll
	}

	var (
		added   = make([]file.Hash, 0)
		changed = make([]file.Hash, 0)
	)

	// find the previous flavor version. since versions are ordered latest -> oldest
	// we can simply add 1 from the current versions index to get the previous one
	idx := slices.Index(remoteFlavor.Versions, remoteVersion)
	if len(remoteFlavor.Versions) > 1 {
		prevVersion := remoteFlavor.Versions[idx+1]
		added, changed, _ = data.local.fileDiff(prevVersion.FileHashes)
	} else {
		// if there are no previous versions, this is the first version ever,
		// so we have to assign all files to 'added'
		added = data.local.files
	}

	// TODO: debug log
	//for _, l := range added {
	//	fmt.Println("added", l)
	//}
	//
	//for _, l := range changed {
	//	fmt.Println("changed", l)
	//}

	files := make([]*os.File, 0, len(data.local.files))
	for _, localFile := range data.local.files {
		isAdded := slices.ContainsFunc(added, func(added file.Hash) bool {
			return added.Hash == localFile.Hash
		})

		isChanged := slices.ContainsFunc(changed, func(changed file.Hash) bool {
			return changed.Hash == localFile.Hash
		})

		if !isAdded && !isChanged {
			continue
		}

		localPath := filepath.Join(data.local.path, localFile.Path)
		f, err := os.Open(localPath)
		if err != nil {
			return fmt.Errorf("error while opening file %s: %w", localPath, err)
		}

		files = append(files, f)
	}

	changeSet := filepath.Join(b.changeSetDir, remoteVersion.Id+".tar.gz")

	if err := tarhelper.TarFiles(data.local.path, files, changeSet); err != nil {
		return fmt.Errorf("error while taring files: %w", err)
	}

	tarData, err := os.ReadFile(changeSet)
	if err != nil {
		return fmt.Errorf("error while reading change set: %w", err)
	}

	progReader := &progressReader{
		size:  float64(len(tarData)),
		inner: bytes.NewReader(tarData),
	}

	digest := sha256.Sum256(tarData)

	uploadURLResp, err := b.client.GetUploadURL(ctx, &chunkv1alpha1.GetUploadURLRequest{
		FlavorVersionId: remoteVersion.Id,
		TarballHash:     base64.StdEncoding.EncodeToString(digest[:]),
	})
	if err != nil {
		return fmt.Errorf("error while getting upload url: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, uploadURLResp.Url, progReader)
	if err != nil {
		return fmt.Errorf("error while creating upload url: %w", err)
	}

	progReader.OnProgress(func(progress uint) {
		b.updates <- buildUpdate{
			data:           *data,
			uploadProgress: &progress,
		}
	})

	uploadResp, err := http.DefaultClient.Do(req)
	if err != nil {
		progReader.StopReporting()
		return fmt.Errorf("error while uploading: %w", err)
	}

	defer uploadResp.Body.Close()

	if uploadResp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(uploadResp.Body)
		if err != nil {
			return fmt.Errorf("error while uploading: unexpected status code: %d", uploadResp.StatusCode)
		}
		fmt.Println(string(body))
		return fmt.Errorf("error while uploading: unexpected status code: %d", uploadResp.StatusCode)
	}

	data.phase = buildPhaseTriggerBuild
	return nil
}

func (b builder) handleTriggerBuild(ctx context.Context, data *buildData) error {
	resp, err := b.client.GetChunk(ctx, &chunkv1alpha1.GetChunkRequest{
		Id: data.chunkID,
	})
	if err != nil {
		return fmt.Errorf("get chunk: %w", err)
	}

	remoteFlavor := cli.Find(resp.Chunk.Flavors, func(item *chunkv1alpha1.Flavor) bool {
		return data.local.name == item.Name
	})

	if remoteFlavor == nil {
		return fmt.Errorf("flavor %s not found in chunk", data.local.name)
	}

	remoteVersion := cli.Find(remoteFlavor.Versions, func(v *chunkv1alpha1.FlavorVersion) bool {
		return data.local.hash == v.Hash
	})

	if remoteVersion == nil {
		return fmt.Errorf("version %s not found for flavor %s", data.local.version, data.local.name)
	}

	if _, err := b.client.BuildFlavorVersion(ctx, &chunkv1alpha1.BuildFlavorVersionRequest{
		FlavorVersionId: remoteVersion.Id,
	}); err != nil {
		return fmt.Errorf("error occured when trying to initate server image build: %w", err)
	}

	data.phase = buildPhaseBuildComplete
	return nil
}
