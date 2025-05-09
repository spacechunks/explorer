// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0
// source: batch.go

package query

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrBatchAlreadyClosed = errors.New("batch already closed")
)

const bulkDeleteInstances = `-- name: BulkDeleteInstances :batchexec
DELETE FROM instances WHERE id = $1
`

type BulkDeleteInstancesBatchResults struct {
	br     pgx.BatchResults
	tot    int
	closed bool
}

func (q *Queries) BulkDeleteInstances(ctx context.Context, id []string) *BulkDeleteInstancesBatchResults {
	batch := &pgx.Batch{}
	for _, a := range id {
		vals := []interface{}{
			a,
		}
		batch.Queue(bulkDeleteInstances, vals...)
	}
	br := q.db.SendBatch(ctx, batch)
	return &BulkDeleteInstancesBatchResults{br, len(id), false}
}

func (b *BulkDeleteInstancesBatchResults) Exec(f func(int, error)) {
	defer b.br.Close()
	for t := 0; t < b.tot; t++ {
		if b.closed {
			if f != nil {
				f(t, ErrBatchAlreadyClosed)
			}
			continue
		}
		_, err := b.br.Exec()
		if f != nil {
			f(t, err)
		}
	}
}

func (b *BulkDeleteInstancesBatchResults) Close() error {
	b.closed = true
	return b.br.Close()
}

const bulkGetBlobData = `-- name: BulkGetBlobData :batchmany
SELECT hash, data, created_at FROM blobs WHERE hash = $1
`

type BulkGetBlobDataBatchResults struct {
	br     pgx.BatchResults
	tot    int
	closed bool
}

func (q *Queries) BulkGetBlobData(ctx context.Context, hash []string) *BulkGetBlobDataBatchResults {
	batch := &pgx.Batch{}
	for _, a := range hash {
		vals := []interface{}{
			a,
		}
		batch.Queue(bulkGetBlobData, vals...)
	}
	br := q.db.SendBatch(ctx, batch)
	return &BulkGetBlobDataBatchResults{br, len(hash), false}
}

func (b *BulkGetBlobDataBatchResults) Query(f func(int, []Blob, error)) {
	defer b.br.Close()
	for t := 0; t < b.tot; t++ {
		var items []Blob
		if b.closed {
			if f != nil {
				f(t, items, ErrBatchAlreadyClosed)
			}
			continue
		}
		err := func() error {
			rows, err := b.br.Query()
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var i Blob
				if err := rows.Scan(&i.Hash, &i.Data, &i.CreatedAt); err != nil {
					return err
				}
				items = append(items, i)
			}
			return rows.Err()
		}()
		if f != nil {
			f(t, items, err)
		}
	}
}

func (b *BulkGetBlobDataBatchResults) Close() error {
	b.closed = true
	return b.br.Close()
}

const bulkInsertBlobData = `-- name: BulkInsertBlobData :batchexec
/*
 * BLOB STORE
 */

INSERT INTO blobs
    (hash, data)
VALUES ($1, $2)
ON CONFLICT DO NOTHING
`

type BulkInsertBlobDataBatchResults struct {
	br     pgx.BatchResults
	tot    int
	closed bool
}

type BulkInsertBlobDataParams struct {
	Hash string
	Data []byte
}

func (q *Queries) BulkInsertBlobData(ctx context.Context, arg []BulkInsertBlobDataParams) *BulkInsertBlobDataBatchResults {
	batch := &pgx.Batch{}
	for _, a := range arg {
		vals := []interface{}{
			a.Hash,
			a.Data,
		}
		batch.Queue(bulkInsertBlobData, vals...)
	}
	br := q.db.SendBatch(ctx, batch)
	return &BulkInsertBlobDataBatchResults{br, len(arg), false}
}

func (b *BulkInsertBlobDataBatchResults) Exec(f func(int, error)) {
	defer b.br.Close()
	for t := 0; t < b.tot; t++ {
		if b.closed {
			if f != nil {
				f(t, ErrBatchAlreadyClosed)
			}
			continue
		}
		_, err := b.br.Exec()
		if f != nil {
			f(t, err)
		}
	}
}

func (b *BulkInsertBlobDataBatchResults) Close() error {
	b.closed = true
	return b.br.Close()
}

const bulkInsertFlavorFileHashes = `-- name: BulkInsertFlavorFileHashes :batchexec
INSERT INTO flavor_version_files
    (flavor_version_id, file_hash, file_path)
VALUES
    ($1, $2, $3)
`

type BulkInsertFlavorFileHashesBatchResults struct {
	br     pgx.BatchResults
	tot    int
	closed bool
}

type BulkInsertFlavorFileHashesParams struct {
	FlavorVersionID string
	FileHash        pgtype.Text
	FilePath        string
}

func (q *Queries) BulkInsertFlavorFileHashes(ctx context.Context, arg []BulkInsertFlavorFileHashesParams) *BulkInsertFlavorFileHashesBatchResults {
	batch := &pgx.Batch{}
	for _, a := range arg {
		vals := []interface{}{
			a.FlavorVersionID,
			a.FileHash,
			a.FilePath,
		}
		batch.Queue(bulkInsertFlavorFileHashes, vals...)
	}
	br := q.db.SendBatch(ctx, batch)
	return &BulkInsertFlavorFileHashesBatchResults{br, len(arg), false}
}

func (b *BulkInsertFlavorFileHashesBatchResults) Exec(f func(int, error)) {
	defer b.br.Close()
	for t := 0; t < b.tot; t++ {
		if b.closed {
			if f != nil {
				f(t, ErrBatchAlreadyClosed)
			}
			continue
		}
		_, err := b.br.Exec()
		if f != nil {
			f(t, err)
		}
	}
}

func (b *BulkInsertFlavorFileHashesBatchResults) Close() error {
	b.closed = true
	return b.br.Close()
}

const bulkUpdateInstanceStateAndPort = `-- name: BulkUpdateInstanceStateAndPort :batchexec
UPDATE instances SET
    state = $1,
    port = $2,
    updated_at = now()
`

type BulkUpdateInstanceStateAndPortBatchResults struct {
	br     pgx.BatchResults
	tot    int
	closed bool
}

type BulkUpdateInstanceStateAndPortParams struct {
	State InstanceState
	Port  *int32
}

func (q *Queries) BulkUpdateInstanceStateAndPort(ctx context.Context, arg []BulkUpdateInstanceStateAndPortParams) *BulkUpdateInstanceStateAndPortBatchResults {
	batch := &pgx.Batch{}
	for _, a := range arg {
		vals := []interface{}{
			a.State,
			a.Port,
		}
		batch.Queue(bulkUpdateInstanceStateAndPort, vals...)
	}
	br := q.db.SendBatch(ctx, batch)
	return &BulkUpdateInstanceStateAndPortBatchResults{br, len(arg), false}
}

func (b *BulkUpdateInstanceStateAndPortBatchResults) Exec(f func(int, error)) {
	defer b.br.Close()
	for t := 0; t < b.tot; t++ {
		if b.closed {
			if f != nil {
				f(t, ErrBatchAlreadyClosed)
			}
			continue
		}
		_, err := b.br.Exec()
		if f != nil {
			f(t, err)
		}
	}
}

func (b *BulkUpdateInstanceStateAndPortBatchResults) Close() error {
	b.closed = true
	return b.br.Close()
}
