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

package blob_test

import (
	"context"
	"testing"

	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/internal/mock"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestBlobStorePut(t *testing.T) {
	tests := []struct {
		name     string
		input    []blob.Object
		expected []blob.Object
		err      error
	}{
		{
			name: "add missing hash",
			input: []blob.Object{
				{
					Hash: "d447b1ea40e6988b",
					Data: []byte("hello world"),
				},
				{
					Data: []byte("ugede ishde"),
				},
			},
			expected: []blob.Object{
				{
					Hash: "d447b1ea40e6988b",
					Data: []byte("hello world"),
				},
				{
					Hash: "1f47515caccc8b7c",
					Data: []byte("ugede ishde"),
				},
			},
		},
		{
			name: "data too large",
			input: []blob.Object{
				{
					Data: make([]byte, 1_000_000_001),
				},
			},
			err: blob.ErrDataTooLarge,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				mockRepo = mock.NewMockBlobRepository(t)
				store    = blob.NewPGStore(mockRepo)
				ctx      = context.Background()
			)

			if tt.err != nil {
				err := store.Put(ctx, tt.input)
				require.ErrorIs(t, err, tt.err)
				return
			}

			mockRepo.EXPECT().
				BulkWriteBlobs(mocky.Anything, tt.expected).
				Return(nil)

			require.NoError(t, store.Put(ctx, tt.input))
		})
	}
}

func TestBlobStoreGet(t *testing.T) {
	var (
		mockRepo = mock.NewMockBlobRepository(t)
		store    = blob.NewPGStore(mockRepo)
		ctx      = context.Background()
		hashes   = []string{"abc", "def"}
		expected = []blob.Object{
			{
				Hash: "abc",
				Data: []byte("hello world"),
			},
			{
				Hash: "def",
				Data: []byte("blabla420"),
			},
		}
	)

	mockRepo.EXPECT().
		BulkGetBlobs(mocky.Anything, hashes).
		Return(expected, nil)

	objs, err := store.Get(ctx, hashes)
	require.NoError(t, err)

	require.Equal(t, expected, objs)
}
