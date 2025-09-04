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

package chunk

import (
	"context"
	"fmt"
	"net/url"
	"time"

	signerv4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	cperrs "github.com/spacechunks/explorer/controlplane/errors"
)

type s3Client interface {
	PresignPutObject(
		ctx context.Context,
		params *s3.PutObjectInput,
		optFns ...func(*s3.PresignOptions),
	) (*signerv4.PresignedHTTPRequest, error)
}

func (s *svc) GetUploadURL(ctx context.Context, flavorVersionID string, tarballHash string) (string, error) {
	key := fmt.Sprintf("explorer/flavor-versions/%s/changeset.tgz", flavorVersionID)

	ver, err := s.repo.FlavorVersionByID(ctx, flavorVersionID)
	if err != nil {
		return "", fmt.Errorf("flavor version: %w", err)
	}

	if ver.FilesUploaded {
		return "", cperrs.ErrFlavorFilesUploaded
	}

	if ver.PresignedURLExpiryDate != nil && time.Now().Before(*ver.PresignedURLExpiryDate) {
		return *ver.PresignedURL, nil
	}

	req, err := s.s3Client.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:            &s.cfg.Bucket,
		Key:               &key,
		ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
		ChecksumSHA256:    &tarballHash,
	}, s3.WithPresignExpires(s.cfg.PresignedURLExpiry))
	if err != nil {
		return "", fmt.Errorf("presign: %w", err)
	}

	reqURL, err := url.Parse(req.URL)
	if err != nil {
		return "", fmt.Errorf("parse url: %w", err)
	}

	date, err := time.Parse("20060102T150405Z", reqURL.Query().Get("X-Amz-Date"))
	if err != nil {
		return "", fmt.Errorf("parse date: %w", err)
	}

	if err := s.repo.UpdateFlavorVersionPresignedURLData(
		ctx,
		flavorVersionID,
		date.Add(s.cfg.PresignedURLExpiry),
		req.URL,
	); err != nil {
		return "", fmt.Errorf("update presigned url data: %w", err)
	}

	return req.URL, nil
}
