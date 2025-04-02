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
	"crypto/sha256"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/cbergoon/merkletree"
)

var (
	ErrFlavorVersionExists       = errors.New("flavor version already exists")
	ErrFlavorVersionHashMismatch = errors.New("flavor hash does not match")
)

type ErrFlavorVersionDuplicate struct {
	// the flavor version that contains the duplicated files
	Version string
}

func (e ErrFlavorVersionDuplicate) Error() string {
	return fmt.Sprintf("flavor version is duplicate of: %s", e.Version)
}

/*
 * flavor types
 */

type Flavor struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type FlavorVersionDiff struct {
	Added   []FileHash
	Removed []FileHash
	Changed []FileHash
}

type FlavorVersion struct {
	ID         string
	Flavor     Flavor
	Version    string
	Hash       string
	FileHashes []FileHash
	CreatedAt  time.Time
}

type FileHash struct {
	Path string
	Hash string
}

func (f FileHash) CalculateHash() ([]byte, error) {
	h := sha256.New()
	if _, err := h.Write([]byte(f.Hash)); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

func (f FileHash) Equals(other merkletree.Content) (bool, error) {
	otherHash, ok := other.(FileHash)
	if !ok {
		return false, errors.New("value is not of type FileHash")
	}
	return f.Hash == otherHash.Hash, nil
}

/*
 * service functions
 */

func (s *svc) CreateFlavorVersion(
	ctx context.Context,
	version FlavorVersion,
) (FlavorVersion, FlavorVersionDiff, error) {
	exists, err := s.repo.FlavorVersionExists(ctx, version.Flavor.ID, version.Version)
	if err != nil {
		return FlavorVersion{}, FlavorVersionDiff{}, fmt.Errorf("flavor version exists: %w", err)
	}

	if exists {
		return FlavorVersion{}, FlavorVersionDiff{}, ErrFlavorVersionExists
	}

	dupVersion, err := s.repo.FlavorVersionByHash(ctx, version.Hash)
	if err != nil {
		return FlavorVersion{}, FlavorVersionDiff{}, err
	}

	if dupVersion != "" {
		return FlavorVersion{}, FlavorVersionDiff{}, ErrFlavorVersionDuplicate{
			Version: dupVersion,
		}
	}

	prevVersion, err := s.repo.LatestFlavorVersion(ctx, version.Flavor.ID)
	if err != nil {
		return FlavorVersion{}, FlavorVersionDiff{}, fmt.Errorf("latest flavor version file hashes: %w", err)
	}

	var (
		prevContent, _             = hashTreeContent(prevVersion.FileHashes)
		newContent, newContentList = hashTreeContent(version.FileHashes)
	)

	newContentTree, err := merkletree.NewTree(newContentList)
	if err != nil {
		return FlavorVersion{}, FlavorVersionDiff{}, fmt.Errorf("merkle tree: %w", err)
	}

	computedHash := fmt.Sprintf("%x", string(newContentTree.MerkleRoot()))
	if computedHash != version.Hash {
		return FlavorVersion{}, FlavorVersionDiff{}, ErrFlavorVersionHashMismatch
	}

	var (
		unchanged = make([]FileHash, 0)
		changed   = make([]FileHash, 0)
		added     = make([]FileHash, 0)
		removed   = make([]FileHash, 0)
	)

	for _, prev := range slices.Collect(maps.Values(prevContent)) {
		ok, err := newContentTree.VerifyContent(prev)
		if err != nil {
			return FlavorVersion{}, FlavorVersionDiff{}, fmt.Errorf("verify content: %w", err)
		}

		prevHash := prev.(FileHash)

		// hash is the same so it is unchanged
		if ok {
			unchanged = append(unchanged, prevHash)
			continue
		}

		// hash differs, but file was already present
		// in the previous version -> it has been changed.
		newFH, found := newContent[prevHash.Path]
		if found {
			changed = append(changed, newFH.(FileHash))
		}

		if !found {
			removed = append(added, prevHash)
		}
	}

	// everything that is contained in the new version,
	// but not found in the previous version, we consider
	// as newly added.
	for _, nc := range newContent {
		fh := nc.(FileHash)
		if _, ok := prevContent[fh.Path]; !ok {
			added = append(added, fh)
		}
	}

	var (
		all  = make([]FileHash, 0, len(unchanged)+len(changed)+len(added))
		diff = FlavorVersionDiff{
			Added:   added,
			Removed: removed,
			Changed: changed,
		}
	)

	all = append(all, unchanged...)
	all = append(all, changed...)
	all = append(all, added...)
	version.FileHashes = all

	created, err := s.repo.CreateFlavorVersion(ctx, version, prevVersion.ID)
	if err != nil {
		return FlavorVersion{}, FlavorVersionDiff{}, fmt.Errorf("create flavor version: %w", err)
	}

	return created, diff, nil
}

func hashTreeContent(hashes []FileHash) (map[string]merkletree.Content, []merkletree.Content) {
	list := make([]merkletree.Content, 0, len(hashes))
	m := make(map[string]merkletree.Content, len(hashes))
	for _, h := range hashes {
		list = append(list, h)
		m[h.Path] = h
	}
	return m, list
}
