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
	"errors"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/spacechunks/explorer/internal/ptr"
)

type S3Store interface {
	PresignURL(
		ctx context.Context,
		key string,
		contentHash string,
		expiry time.Duration) (string, time.Time, error)
	WriteTo(ctx context.Context, key string, w io.Writer) error
	ObjectExists(ctx context.Context, key string) (bool, error)
	Put(ctx context.Context, keyPrefix string, objects []Object) error
	SimplePut(ctx context.Context, key string, r io.Reader, metadata map[string]string) error
}

type S3ObjectStore struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
}

func NewS3Store(bucket string, c *s3.Client, presigner *s3.PresignClient) *S3ObjectStore {
	return &S3ObjectStore{
		client:    c,
		presigner: presigner,
		bucket:    bucket,
	}
}

func (s S3ObjectStore) PresignURL(
	ctx context.Context,
	key string,
	contentHash string,
	expiry time.Duration,
) (string, time.Time, error) {
	// TODO: add content length to prevent users from uploading too large files
	req, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:            &s.bucket,
		Key:               &key,
		ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
		ChecksumSHA256:    &contentHash,
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("presign: %w", err)
	}

	reqURL, err := url.Parse(req.URL)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("parse url: %w", err)
	}

	date, err := time.Parse("20060102T150405Z", reqURL.Query().Get("X-Amz-Date"))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("parse date: %w", err)
	}

	return req.URL, date.Add(expiry), nil
}

func (s S3ObjectStore) ObjectExists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	if err != nil {
		var s3err smithy.APIError
		if errors.As(err, &s3err) && s3err.ErrorCode() == "NotFound" {
			return false, nil
		}
		return false, fmt.Errorf("head object: %w", err)
	}

	return true, nil
}

func (s S3ObjectStore) WriteTo(ctx context.Context, key string, w io.Writer) error {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	if err != nil {
		return fmt.Errorf("get object: %w", err)
	}

	_, err = io.Copy(w, out.Body)
	if err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	return nil
}

// Put uploads all the given objects to S3. Note that the objects
// underlying io.ReadSeekCloser will be closed after it has been uploaded.
func (s S3ObjectStore) Put(ctx context.Context, keyPrefix string, objects []Object) error {
	uploader := manager.NewUploader(s.client)

	for _, obj := range objects {
		h, err := obj.Hash()
		if err != nil {
			return fmt.Errorf("hash: %w", err)
		}

		found, err := s.ObjectExists(ctx, keyPrefix+"/"+h)
		if err != nil {
			return fmt.Errorf("exists: %w", err)
		}

		if found {
			continue
		}

		if _, err := uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket: &s.bucket,
			Key:    ptr.Pointer(keyPrefix + "/" + h),
			Body:   obj.Data,
		}); err != nil {
			return fmt.Errorf("upload: %w", err)
		}

		obj.Data.Close()
	}

	return nil
}

func (s S3ObjectStore) SimplePut(ctx context.Context, key string, r io.Reader, metadata map[string]string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:   &s.bucket,
		Key:      &key,
		Body:     r,
		Metadata: metadata,
	})
	return err
}
