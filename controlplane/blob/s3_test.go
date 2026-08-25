/*
A basic matchmaking service for the Chunk Explorer.
Copyright (C) 2026 Yannic Rieger <oss@76k.io>

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
	"net/url"
	"testing"
	"time"

	awssignerv4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/internal/mock"
	mocky "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPresignURL(t *testing.T) {
	var (
		ctx           = context.Background()
		bucket        = "bucket"
		key           = "key"
		contentHash   = "hash"
		expiry        = time.Second * 1
		contentLen    = int64(len("content"))
		mockPresigner = mock.NewMockBlobPresigner(t)
	)

	mockPresigner.EXPECT().PresignPutObject(
		mocky.Anything,
		&s3.PutObjectInput{
			Bucket:            &bucket,
			Key:               &key,
			ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
			ChecksumSHA256:    &contentHash,
			ContentLength:     new(contentLen),
		}, mocky.AnythingOfType("func(*s3.PresignOptions)"),
	).
		Return(&awssignerv4.PresignedHTTPRequest{
			URL: "example.com?X-Amz-Date=" + time.Now().Format("20060102T150405Z"),
		}, nil)

	store := blob.NewS3Store(bucket, nil, mockPresigner)
	rawURL, actualExpiry, err := store.PresignURL(ctx, key, contentHash, expiry, uint64(contentLen))
	require.NoError(t, err)

	u, err := url.Parse(rawURL)
	require.NoError(t, err)

	date, err := time.Parse("20060102T150405Z", u.Query().Get("X-Amz-Date"))
	require.NoError(t, err)

	require.Equal(t, date.Add(expiry), actualExpiry)
}
