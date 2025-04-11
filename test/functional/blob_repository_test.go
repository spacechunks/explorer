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

package functional

import (
	"context"
	"testing"

	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/stretchr/testify/require"
)

func TestBulkInsertBlobs(t *testing.T) {
	var (
		ctx   = context.Background()
		pg    = fixture.NewPostgres()
		input = []blob.Object{
			{
				Hash: "1f47515caccc8b7c",
				Data: []byte("ugede ishde"),
			},
			{
				Hash: "d447b1ea40e6988b",
				Data: []byte("hello world"),
			},
		}
	)

	pg.Run(t, ctx)
	require.NoError(t, pg.DB.BulkWriteBlobs(context.Background(), input))

	rows, err := pg.Pool.Query(ctx, "SELECT hash, data FROM blobs")
	require.NoError(t, err)

	actual := make([]blob.Object, 0)
	for rows.Next() {
		var (
			hash string
			data []byte
		)
		err := rows.Scan(&hash, &data)
		require.NoError(t, err, "scan row")
		actual = append(actual, blob.Object{
			Hash: hash,
			Data: data,
		})
	}
	rows.Close()
	require.NoError(t, rows.Err())

	require.ElementsMatch(t, input, actual)
}

func TestBulkGetBlobs(t *testing.T) {
	var (
		ctx   = context.Background()
		pg    = fixture.NewPostgres()
		input = []blob.Object{
			{
				Hash: "1f47515caccc8b7c",
				Data: []byte("ugede ishde"),
			},
			{
				Hash: "d447b1ea40e6988b",
				Data: []byte("hello world"),
			},
		}
	)

	pg.Run(t, ctx)

	require.NoError(t, pg.DB.BulkWriteBlobs(ctx, input))

	actual, err := pg.DB.BulkGetBlobs(ctx, []string{"d447b1ea40e6988b", "1f47515caccc8b7c"})
	require.NoError(t, err)

	require.ElementsMatch(t, input, actual)
}
