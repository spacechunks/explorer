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
	"sort"
	"strings"

	"github.com/cbergoon/merkletree"
	"github.com/zeebo/xxh3"
)

type Object struct {
	Path string
	Data []byte
}

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

func ComputeHashStr(data []byte) string {
	return fmt.Sprintf("%x", xxh3.Hash(data))
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

func SortFiles(objs []Object) {
	sort.Slice(objs, func(i, j int) bool {
		return strings.Compare(objs[i].Path, objs[j].Path) < 0
	})
}

func SortHashes(hashes []Hash) {
	sort.Slice(hashes, func(i, j int) bool {
		return strings.Compare(hashes[i].Path, hashes[j].Path) < 0
	})
}
