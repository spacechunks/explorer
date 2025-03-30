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
	"crypto/sha256"
	"errors"
	"time"

	"github.com/cbergoon/merkletree"
)

type Flavor struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Chunk struct {
	ID          string
	Name        string
	Description string
	Tags        []string
	Flavors     []Flavor
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
