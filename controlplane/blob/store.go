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

package blob

import (
	"context"
	"fmt"

	"github.com/zeebo/xxh3"
)

type Object struct {
	Hash string
	Data []byte
}

type Store interface {
	Put(ctx context.Context, objects []Object) error
	Get(ctx context.Context, hashes []string) ([]Object, error)
}

type Repository interface {
	BulkWriteBlobs(ctx context.Context, objects []Object) error
	BulkGetBlobs(ctx context.Context, hashes []string) ([]Object, error)
}

type pgStore struct {
	repo Repository
}

func NewPGStore(repo Repository) Store {
	return &pgStore{
		repo: repo,
	}
}

func (s *pgStore) Put(ctx context.Context, objects []Object) error {
	// make sure every passed object has a valid hash set
	// if this is not the case, writing to the db fails
	for i, obj := range objects {
		if obj.Hash != "" {
			continue
		}
		objects[i].Hash = fmt.Sprintf("%x", xxh3.Hash(obj.Data))
	}
	return s.repo.BulkWriteBlobs(ctx, objects)
}

func (s *pgStore) Get(ctx context.Context, hashes []string) ([]Object, error) {
	objs, err := s.repo.BulkGetBlobs(ctx, hashes)
	if err != nil {
		return nil, err
	}
	return objs, nil
}
