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
	"errors"
	"fmt"
	"time"

	"github.com/spacechunks/explorer/controlplane/authz"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/contextkey"
	cperrs "github.com/spacechunks/explorer/controlplane/errors"
)

func (s *svc) GetUploadURL(ctx context.Context, versionID string, tarballHash string) (string, error) {
	actorID, ok := ctx.Value(contextkey.ActorID).(string)
	if !ok {
		return "", errors.New("actor_id not found in context")
	}

	if err := s.access.AccessAuthorized(
		ctx,
		authz.WithOwnershipRule(actorID, authz.FlavorVersionResourceDef(versionID)),
	); err != nil {
		return "", fmt.Errorf("access: %w", err)
	}

	// TODO: tarball size needs to be specified as well to prevent people from uploading too large files
	//       if size > 1GB reject

	ver, err := s.repo.FlavorVersionByID(ctx, versionID)
	if err != nil {
		return "", fmt.Errorf("flavor version: %w", err)
	}

	if ver.FilesUploaded {
		return "", cperrs.ErrFlavorFilesUploaded
	}

	if ver.PresignedURLExpiryDate != nil && time.Now().Before(*ver.PresignedURLExpiryDate) {
		return *ver.PresignedURL, nil
	}

	url, expiryDate, err := s.s3Store.PresignURL(
		ctx,
		blob.ChangeSetKey(versionID),
		tarballHash,
		s.cfg.PresignedURLExpiry,
	)
	if err != nil {
		return "", fmt.Errorf("presign: %w", err)
	}

	if err := s.repo.UpdateFlavorVersionPresignedURLData(
		ctx,
		versionID,
		expiryDate,
		url,
	); err != nil {
		return "", fmt.Errorf("update presigned url data: %w", err)
	}

	return url, nil
}
