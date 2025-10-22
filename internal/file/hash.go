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

// Package file provides structs for a common way to represent files.
package file

import (
	"errors"
	"fmt"
	"hash"
	"io"
	"sort"
	"strings"

	"github.com/cbergoon/merkletree"
	"github.com/zeebo/xxh3"
)

type Hash struct {
	Path string
	Hash string
}

func (f Hash) CalculateHash() ([]byte, error) {
	return []byte(f.Hash), nil
}

func (f Hash) Equals(other merkletree.Content) (bool, error) {
	otherHash, ok := other.(Hash)
	if !ok {
		return false, errors.New("value is not of type Hash")
	}
	return f.Hash == otherHash.Hash, nil
}

// ComputeHashStr computes a xxh3 hash from the given io.ReadSeekCloser.
// this is designed to be able to be used with large amounts of data.
func ComputeHashStr(file io.ReadSeekCloser) (string, error) {
	// in case we have a large file, we don't want to read the whole thing into memory.
	hasher := xxh3.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("copy: %w", err)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek: %w", err)
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func HashTreeRootString(tree *merkletree.MerkleTree) string {
	return fmt.Sprintf("%x", tree.MerkleRoot())
}

func HashTree[T merkletree.Content](hashes []T) (*merkletree.MerkleTree, error) {
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

func SortHashes(hashes []Hash) {
	sort.Slice(hashes, func(i, j int) bool {
		return strings.Compare(hashes[i].Path, hashes[j].Path) < 0
	})
}
