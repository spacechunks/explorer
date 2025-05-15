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

	"github.com/jackc/pgx/v5"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/postgres/query"
)

func (db *DB) BulkWriteBlobs(ctx context.Context, objects []blob.Object) error {
	return db.doTX(ctx, func(tx pgx.Tx, q *query.Queries) error {
		objs := make([]query.BulkInsertBlobDataParams, 0, len(objects))
		for _, o := range objects {
			objs = append(objs, query.BulkInsertBlobDataParams{
				Hash: o.Hash,
				Data: o.Data,
			})
		}
		return db.bulkExecAndClose(q.BulkInsertBlobData(ctx, objs))
	})
}

func (db *DB) BulkGetBlobs(ctx context.Context, hashes []string) ([]blob.Object, error) {
	var ret []blob.Object
	if err := db.do(ctx, func(q *query.Queries) error {
		ret = make([]blob.Object, 0, len(hashes))
		res := q.BulkGetBlobData(ctx, hashes)

		defer res.Close()

		var err error
		res.Query(func(i int, blobs []query.Blob, inner error) {
			if err != nil {
				err = inner
				return
			}

			for _, b := range blobs {
				ret = append(ret, blob.Object{
					Hash: b.Hash,
					Data: b.Data,
				})
			}
		})

		if err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return ret, nil
}
