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
	"time"

	cperrs "github.com/spacechunks/explorer/controlplane/errors"
)

func (s *svc) GetUploadURL(ctx context.Context, flavorVersionID string, tarballHash string) (string, error) {
	// TODO: tarball size needs to be specified as well to prevent people from uploading too large files
	//       if size > 1GB reject

	key := fmt.Sprintf("explorer/flavor-versions/%s/changeset.tar.gz", flavorVersionID)

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

	url, expiryDate, err := s.s3Store.PresignURL(ctx, key, tarballHash, s.cfg.PresignedURLExpiry)
	if err != nil {
		return "", fmt.Errorf("presign: %w", err)
	}

	if err := s.repo.UpdateFlavorVersionPresignedURLData(
		ctx,
		flavorVersionID,
		expiryDate,
		url,
	); err != nil {
		return "", fmt.Errorf("update presigned url data: %w", err)
	}

	return url, nil
}
