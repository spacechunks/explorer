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
	"fmt"
	"hash"
	"log"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/cbergoon/merkletree"
	"github.com/spacechunks/explorer/controlplane/blob"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/file"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/zeebo/xxh3"
)

/*
 * flavor types
 */

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
	ID            string
	Version       string
	Hash          string
	ChangeHash    string
	FileHashes    []file.Hash
	FilesUploaded bool
	BuildStatus   BuildStatus
	CreatedAt     time.Time
}

/*
 * service functions
 */

func (s *svc) CreateFlavor(ctx context.Context, chunkID string, flavor Flavor) (Flavor, error) {
	exists, err := s.repo.FlavorNameExists(ctx, chunkID, flavor.Name)
	if err != nil {
		return Flavor{}, fmt.Errorf("flavor name exists: %w", err)
	}

	if exists {
		return Flavor{}, apierrs.ErrFlavorNameExists
	}

	ret, err := s.repo.CreateFlavor(ctx, chunkID, flavor)
	if err != nil {
		return Flavor{}, fmt.Errorf("create flavor: %w", err)
	}

	return ret, nil
}

func (s *svc) CreateFlavorVersion(
	ctx context.Context,
	flavorID string,
	version FlavorVersion,
) (FlavorVersion, FlavorVersionDiff, error) {
	exists, err := s.repo.FlavorVersionExists(ctx, flavorID, version.Version)
	if err != nil {
		return FlavorVersion{}, FlavorVersionDiff{}, fmt.Errorf("flavor version exists: %w", err)
	}

	if exists {
		return FlavorVersion{}, FlavorVersionDiff{}, apierrs.ErrFlavorVersionExists
	}

	dupVersion, err := s.repo.FlavorVersionByHash(ctx, version.Hash)
	if err != nil {
		return FlavorVersion{}, FlavorVersionDiff{}, fmt.Errorf("flavor version by hash: %w", err)
	}

	if dupVersion != "" {
		return FlavorVersion{}, FlavorVersionDiff{}, apierrs.FlavorVersionDuplicate(dupVersion)
	}

	prevVersion, err := s.repo.LatestFlavorVersion(ctx, flavorID)
	if err != nil {
		return FlavorVersion{}, FlavorVersionDiff{}, fmt.Errorf("latest flavor version file hashes: %w", err)
	}

	var (
		prevContent = contentMap(prevVersion.FileHashes)
		newContent  = contentMap(version.FileHashes)
	)

	newContentTree, err := tree(version.FileHashes)
	if err != nil {
		return FlavorVersion{}, FlavorVersionDiff{}, fmt.Errorf("new content tree: %w", err)
	}

	if hashString(newContentTree) != version.Hash {
		return FlavorVersion{}, FlavorVersionDiff{}, apierrs.ErrHashMismatch
	}

	var (
		unchanged = make([]file.Hash, 0)
		changed   = make([]file.Hash, 0)
		added     = make([]file.Hash, 0)
		removed   = make([]file.Hash, 0)
	)

	for _, prev := range slices.Collect(maps.Values(prevContent)) {
		ok, err := newContentTree.VerifyContent(prev)
		if err != nil {
			return FlavorVersion{}, FlavorVersionDiff{}, fmt.Errorf("verify content: %w", err)
		}

		prevHash := prev.(file.Hash)

		// hash is the same so it is unchanged
		if ok {
			unchanged = append(unchanged, prevHash)
			continue
		}

		// hash differs, but file was already present
		// in the previous version -> it has been changed.
		newFH, found := newContent[prevHash.Path]
		if found {
			changed = append(changed, newFH.(file.Hash))
		}

		if !found {
			removed = append(added, prevHash)
		}
	}

	// everything that is contained in the new version,
	// but not found in the previous version, we consider
	// as newly added.
	for _, nc := range newContent {
		fh := nc.(file.Hash)
		if _, ok := prevContent[fh.Path]; !ok {
			added = append(added, fh)
		}
	}

	var (
		all  = make([]file.Hash, 0, len(unchanged)+len(changed)+len(added))
		diff = FlavorVersionDiff{
			Added:   added,
			Removed: removed,
			Changed: changed,
		}
		sortByPath = func(sl []file.Hash) {
			sort.Slice(sl, func(i, j int) bool {
				return strings.Compare(sl[i].Path, sl[j].Path) < 0
			})
		}
	)

	sortByPath(unchanged)
	sortByPath(changed)
	sortByPath(added)
	sortByPath(removed)

	changes := make([]file.Hash, 0, len(changed)+len(added))
	changes = append(changes, changed...)
	changes = append(changes, added...)
	sortByPath(changes)

	for _, c := range changes {
		log.Println("change: ", c.Path)
	}

	all = append(all, changed...)
	all = append(all, added...)
	all = append(all, unchanged...)

	sortByPath(all)

	changesTree, err := tree(changes)
	if err != nil {
		return FlavorVersion{}, FlavorVersionDiff{}, fmt.Errorf("changes tree: %w", err)
	}

	version.ChangeHash = hashString(changesTree)
	version.FileHashes = all

	created, err := s.repo.CreateFlavorVersion(ctx, flavorID, version, prevVersion.ID)
	if err != nil {
		return FlavorVersion{}, FlavorVersionDiff{}, fmt.Errorf("create flavor version: %w", err)
	}

	return created, diff, nil
}

func (s *svc) SaveFlavorFiles(ctx context.Context, versionID string, files []file.Object) error {
	version, err := s.repo.FlavorVersionByID(ctx, versionID)
	if err != nil {
		return fmt.Errorf("flavor version: %w", err)
	}

	sort.Slice(files, func(i, j int) bool {
		return strings.Compare(files[i].Path, files[j].Path) < 0
	})

	objs := make([]blob.Object, 0, len(files))
	for _, file := range files {
		fmt.Println("file: ", file.Path)
		objs = append(objs, blob.Object{
			Data: file.Data,
		})
	}

	// FIXME: at some point refactor file upload to detect directly if
	//        files have already been uploaded. currently only after
	//       all files have been uploaded this check will be executed.
	if version.FilesUploaded {
		return apierrs.ErrFilesAlreadyExist
	}

	tree, err := tree(objs)
	if err != nil {
		return fmt.Errorf("tree files: %w", err)
	}

	fmt.Println("got: " + hashString(tree))
	fmt.Println("want: " + version.ChangeHash)

	if hashString(tree) != version.ChangeHash {
		return apierrs.ErrHashMismatch
	}

	if err := s.blobStore.Put(ctx, objs); err != nil {
		return fmt.Errorf("put files: %w", err)
	}

	if err := s.repo.MarkFlavorVersionFilesUploaded(ctx, versionID); err != nil {
		return fmt.Errorf("mark flavor version: %w", err)
	}

	return nil
}

func (s *svc) BuildFlavorVersion(ctx context.Context, versionID string) error {
	version, err := s.repo.FlavorVersionByID(ctx, versionID)
	if err != nil {
		return fmt.Errorf("flavor version: %w", err)
	}

	if !version.FilesUploaded {
		return apierrs.ErrFlavorFilesNotUploaded
	}

	// do not fail the request if there is already a job running,
	// or it is already completed, because those states do not
	// indicate that anything is wrong.
	if version.BuildStatus == BuildStatusBuildCheckpoint ||
		version.BuildStatus == BuildStatusBuildImage ||
		version.BuildStatus == BuildStatusCompleted {
		return nil
	}

	// TODO: if previous state is CHECKPOINT_FAILED => start checkpoint job otherwise start create image job

	if err := s.jobClient.InsertJob(ctx, versionID, string(BuildStatusBuildImage), job.CreateImage{
		FlavorVersionID: versionID,
		BaseImage:       s.baseImage,
		OCIRegistry:     s.registry,
	}); err != nil {
		return fmt.Errorf("insert create image job: %w", err)
	}

	return nil
}

func hashString(tree *merkletree.MerkleTree) string {
	return fmt.Sprintf("%x", tree.MerkleRoot())
}

func tree[T merkletree.Content](hashes []T) (*merkletree.MerkleTree, error) {
	sl := make([]merkletree.Content, 0, len(hashes))
	for _, h := range hashes {
		sl = append(sl, h)
	}
	tree, err := merkletree.NewTreeWithHashStrategy(sl, func() hash.Hash {
		return xxh3.New()
	})
	if err != nil {
		return nil, err
	}
	return tree, nil
}

func contentMap(hashes []file.Hash) map[string]merkletree.Content {
	m := make(map[string]merkletree.Content, len(hashes))
	for _, h := range hashes {
		m[h.Path] = h
	}
	return m
}
