package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/spacechunks/explorer/controlplane/postgres/query"
	"github.com/spacechunks/explorer/controlplane/resource"
)

func (db *DB) ArchiveChunk(ctx context.Context, chunk resource.Chunk) error {
	return db.doTX(ctx, func(tx pgx.Tx, q *query.Queries) error {
		data, err := json.Marshal(chunk)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}

		if err := q.ArchiveChunk(ctx, query.ArchiveChunkParams{
			ID:        chunk.ID,
			OwnerID:   chunk.Owner.ID,
			Data:      data,
			CreatedAt: time.Now(),
		}); err != nil {
			return fmt.Errorf("archive: %w", err)
		}

		if err := q.DeleteChunk(ctx, chunk.ID); err != nil {
			return fmt.Errorf("delete: %w", err)
		}

		return nil
	})
}

func (db *DB) ArchiveFlavor(ctx context.Context, chunkID string, flavor resource.Flavor) error {
	return db.doTX(ctx, func(tx pgx.Tx, q *query.Queries) error {
		data, err := json.Marshal(flavor)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}

		if err := q.ArchiveFlavor(ctx, query.ArchiveFlavorParams{
			ID:        flavor.ID,
			ChunkID:   chunkID,
			Data:      data,
			CreatedAt: time.Now(),
		}); err != nil {
			return fmt.Errorf("archive: %w", err)
		}

		if err := q.DeleteFlavor(ctx, flavor.ID); err != nil {
			return fmt.Errorf("delete: %w", err)
		}

		return nil
	})
}

func (db *DB) ArchiveFlavorVersion(ctx context.Context, flavorID string, version resource.FlavorVersion) error {
	return db.doTX(ctx, func(tx pgx.Tx, q *query.Queries) error {
		data, err := json.Marshal(version)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}

		if err := q.ArchiveFlavorVersion(ctx, query.ArchiveFlavorVersionParams{
			ID:        version.ID,
			FlavorID:  flavorID,
			Data:      data,
			CreatedAt: time.Now(),
		}); err != nil {
			return fmt.Errorf("archive: %w", err)
		}

		if err := q.DeleteFlavorVersion(ctx, version.ID); err != nil {
			return fmt.Errorf("delete: %w", err)
		}

		return nil
	})
}
