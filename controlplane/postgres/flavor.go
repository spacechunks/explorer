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
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/postgres/query"
)

type flavorParams struct {
	create query.CreateFlavorParams
}

func createFlavorParams(flavor chunk.Flavor, chunkID string) flavorParams {
	return flavorParams{
		create: query.CreateFlavorParams{
			ID:        flavor.ID,
			ChunkID:   chunkID,
			Name:      flavor.Name,
			CreatedAt: flavor.CreatedAt,
			UpdatedAt: flavor.UpdatedAt,
		},
	}
}

func (db *DB) CreateFlavor(ctx context.Context, flavor chunk.Flavor, chunkID string) (chunk.Flavor, error) {
	params := createFlavorParams(flavor, chunkID)

	var ret chunk.Flavor
	if err := db.do(ctx, func(q *query.Queries) error {
		f, err := q.CreateFlavor(ctx, params.create)
		if err != nil {
			return fmt.Errorf("create flavor: %w", err)
		}

		ret = chunk.Flavor{
			ID:        f.ID,
			Name:      f.Name,
			CreatedAt: f.CreatedAt.UTC(),
			UpdatedAt: f.UpdatedAt.UTC(),
		}
		return nil
	}); err != nil {
		return chunk.Flavor{}, err
	}

	return ret, nil
}

func (db *DB) FlavorVersionExists(ctx context.Context, flavorID string, version string) (bool, error) {
	if err := db.do(ctx, func(q *query.Queries) error {
		return q.FlavorVersionExists(ctx, query.FlavorVersionExistsParams{
			FlavorID: flavorID,
			Version:  version,
		})
	}); err != nil {
		return false, err
	}

	return true, nil
}

func (db *DB) FlavorVersionByHash(ctx context.Context, hash string) (string, error) {
	var ret string
	if err := db.do(ctx, func(q *query.Queries) error {
		version, err := q.FlavorVersionByHash(ctx, hash)
		if err != nil {
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
			return fmt.Errorf("get flavor version: %w", err)
		}

		files, err := q.FlavorVersionFileHashes(ctx, latest.FlavorID)
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

	if err := db.doTX(ctx, func(q *query.Queries) error {
		if err := q.CreateFlavorVersion(ctx, query.CreateFlavorVersionParams{
			ID:            id.String(),
			FlavorID:      version.Flavor.ID,
			Hash:          version.Hash,
			Version:       version.Version,
			PrevVersionID: prevVersionID,
			CreatedAt:     time.Now(),
		}); err != nil {
			return fmt.Errorf("create flavor version: %w", err)
		}

		dbHashes := make([]query.BulkInsertFlavorFileHashesParams, 0, len(version.FileHashes))
		for _, f := range version.FileHashes {
			dbHashes = append(dbHashes, query.BulkInsertFlavorFileHashesParams{
				FlavorVersionID: version.ID,
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
	ret.CreatedAt = version.CreatedAt

	return ret, nil
}
