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

package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/postgres/query"
)

func (db *DB) CreateFlavor(ctx context.Context, chunkID string, flavor chunk.Flavor) (chunk.Flavor, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return chunk.Flavor{}, fmt.Errorf("create flavor id: %w", err)
	}

	var ret chunk.Flavor
	if err := db.do(ctx, func(q *query.Queries) error {
		now := time.Now()

		if err := q.CreateFlavor(ctx, query.CreateFlavorParams{
			ID:        id.String(),
			ChunkID:   chunkID,
			Name:      flavor.Name,
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			return fmt.Errorf("create flavor: %w", err)
		}

		ret = chunk.Flavor{
			ID:        id.String(),
			Name:      flavor.Name,
			CreatedAt: now,
			UpdatedAt: now,
		}

		return nil
	}); err != nil {
		return chunk.Flavor{}, err
	}

	return ret, nil
}

func (db *DB) ListFlavorsByChunkID(ctx context.Context, chunkID string) ([]chunk.Flavor, error) {
	var ret []chunk.Flavor
	if err := db.do(ctx, func(q *query.Queries) error {
		flavors, err := q.ListFlavorsByChunkID(ctx, chunkID)
		if err != nil {
			return err
		}

		ret = make([]chunk.Flavor, 0, len(flavors))

		for _, f := range flavors {
			ret = append(ret, chunk.Flavor{
				ID:        f.ID,
				Name:      f.Name,
				CreatedAt: f.CreatedAt,
				UpdatedAt: f.UpdatedAt,
			})
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (db *DB) FlavorNameExists(ctx context.Context, chunkID string, name string) (bool, error) {
	var ret bool
	if err := db.do(ctx, func(q *query.Queries) error {
		ok, err := q.FlavorNameExists(ctx, query.FlavorNameExistsParams{
			ChunkID: chunkID,
			Name:    name,
		})
		if err != nil {
			return err
		}

		ret = ok
		return nil
	}); err != nil {
		return false, err
	}

	return ret, nil
}

func (db *DB) FlavorVersionExists(ctx context.Context, flavorID string, version string) (bool, error) {
	var ret bool
	if err := db.do(ctx, func(q *query.Queries) error {
		ok, err := q.FlavorVersionExists(ctx, query.FlavorVersionExistsParams{
			FlavorID: flavorID,
			Version:  version,
		})
		if err != nil {
			return err
		}

		ret = ok
		return nil
	}); err != nil {
		return false, err
	}

	return ret, nil
}

func (db *DB) FlavorVersionByHash(ctx context.Context, hash string) (string, error) {
	var ret string
	if err := db.do(ctx, func(q *query.Queries) error {
		version, err := q.FlavorVersionByHash(ctx, hash)
		if err != nil {
			// if no row is found this means we are fine
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		ret = version
		return nil
	}); err != nil {
		return "", err
	}

	return ret, nil
}

func (db *DB) LatestFlavorVersion(ctx context.Context, flavorID string) (chunk.FlavorVersion, error) {
	var ret chunk.FlavorVersion
	if err := db.doTX(ctx, func(q *query.Queries) error {
		// FIXME: at some point join with flavors table to return the complete
		// FlavorVersion object, right now there is no need so skip this

		latest, err := q.LatestFlavorVersionByFlavorID(ctx, flavorID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return fmt.Errorf("get flavor version: %w", err)
		}

		files, err := q.FlavorVersionFileHashes(ctx, latest.ID)
		if err != nil {
			return err
		}

		hashes := make([]chunk.FileHash, 0, len(files))
		for _, f := range files {
			hashes = append(hashes, chunk.FileHash{
				Path: f.FilePath,
				Hash: f.FileHash.String,
			})
		}

		sort.Slice(hashes, func(i, j int) bool {
			return strings.Compare(hashes[i].Path, hashes[j].Path) < 0
		})

		ret = chunk.FlavorVersion{
			ID: latest.ID,
			Flavor: chunk.Flavor{
				ID: latest.FlavorID,
			},
			Version:    latest.Version,
			Hash:       latest.Hash,
			FileHashes: hashes,
		}

		return nil
	}); err != nil {
		return chunk.FlavorVersion{}, err
	}

	return ret, nil
}

func (db *DB) CreateFlavorVersion(
	ctx context.Context,
	version chunk.FlavorVersion,
	prevVersionID string,
) (chunk.FlavorVersion, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return chunk.FlavorVersion{}, fmt.Errorf("flavor version id: %w", err)
	}

	now := time.Now()

	if err := db.doTX(ctx, func(q *query.Queries) error {
		createParams := query.CreateFlavorVersionParams{
			ID:        id.String(),
			FlavorID:  version.Flavor.ID,
			Hash:      version.Hash,
			Version:   version.Version,
			CreatedAt: now,
		}

		if prevVersionID != "" {
			createParams.PrevVersionID = &prevVersionID
		}

		if err := q.CreateFlavorVersion(ctx, createParams); err != nil {
			return fmt.Errorf("create flavor version: %w", err)
		}

		dbHashes := make([]query.BulkInsertFlavorFileHashesParams, 0, len(version.FileHashes))
		for _, f := range version.FileHashes {
			dbHashes = append(dbHashes, query.BulkInsertFlavorFileHashesParams{
				FlavorVersionID: id.String(),
				FileHash: pgtype.Text{
					String: f.Hash,
					Valid:  true,
				},
				FilePath: f.Path,
			})
		}

		if err := db.bulkExecAndClose(q.BulkInsertFlavorFileHashes(ctx, dbHashes)); err != nil {
			return fmt.Errorf("bulk insert flavor files: %w", err)
		}

		return nil
	}); err != nil {
		return chunk.FlavorVersion{}, err
	}

	ret := version
	ret.ID = id.String()
	ret.CreatedAt = now

	return ret, nil
}
