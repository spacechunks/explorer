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
	"sort"
	"strings"
	"time"

	"github.com/spacechunks/explorer/controlplane/authz"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/contextkey"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/internal/file"
	"github.com/spacechunks/explorer/internal/resource"
)

func (s *svc) GetUploadURL(
	ctx context.Context,
	versionID string,
	tarballHash string,
	tarballSizeBytes uint64,
) (string, error) {
	actorID, ok := ctx.Value(contextkey.ActorIDPID).(string)
	if !ok {
		return "", errors.New("actor_id not found in context")
	}

	if err := s.access.AccessAuthorized(
		ctx,
		authz.WithOwnershipRule(actorID, authz.FlavorVersionResourceDef(versionID)),
	); err != nil {
		return "", fmt.Errorf("access: %w", err)
	}

	if tarballSizeBytes > s.cfg.ChangesetTarballMaxSizeBytes {
		return "", apierrs.ErrChangeSetTarballTooBig
	}

	ver, err := s.repo.FlavorVersionByID(ctx, versionID)
	if err != nil {
		return "", fmt.Errorf("flavor version: %w", err)
	}

	if ver.BuildStatus == resource.FlavorVersionBuildStatusFilesVerification {
		return "", apierrs.ErrFlavorVersionVerifying
	}

	missing, err := s.filesToUpload(ctx, ver)
	if err != nil {
		return "", fmt.Errorf("files to upload: %w", err)
	}

	if len(missing) == 0 {
		return "", apierrs.ErrFlavorFilesUploaded
	}

	if ver.PresignedURLExpiryDate != nil && time.Now().Before(*ver.PresignedURLExpiryDate) {
		return *ver.PresignedURL, nil
	}

	url, expiryDate, err := s.s3Store.PresignURL(
		ctx,
		blob.ChangeSetKey(versionID),
		tarballHash,
		s.cfg.PresignedURLExpiry,
		tarballSizeBytes,
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

// files the client still has to upload, because we don't know
// them to be in the blob store yet.
func (s *svc) filesToUpload(ctx context.Context, version resource.FlavorVersion) ([]file.Hash, error) {
	hashes := make([]string, 0, len(version.FileHashes))
	for _, fh := range version.FileHashes {
		hashes = append(hashes, fh.Hash)
	}

	existing, err := s.repo.ExistingBlobHashes(ctx, hashes)
	if err != nil {
		return nil, fmt.Errorf("existing blob hashes: %w", err)
	}

	missing := make([]file.Hash, 0)
	for _, fh := range version.FileHashes {
		if _, ok := existing[fh.Hash]; ok {
			continue
		}
		missing = append(missing, fh)
	}

	sort.Slice(missing, func(i, j int) bool {
		return strings.Compare(missing[i].Path, missing[j].Path) < 0
	})

	return missing, nil
}

func (s *svc) GetFilesToUpload(ctx context.Context, versionID string) ([]file.Hash, error) {
	actorID, ok := ctx.Value(contextkey.ActorIDPID).(string)
	if !ok {
		return nil, errors.New("actor_id not found in context")
	}

	if err := s.access.AccessAuthorized(
		ctx,
		authz.WithOwnershipRule(actorID, authz.FlavorVersionResourceDef(versionID)),
	); err != nil {
		return nil, fmt.Errorf("access: %w", err)
	}

	flavorID, err := s.repo.FlavorIDByFlavorVersionID(ctx, versionID)
	if err != nil {
		return nil, fmt.Errorf("get flavor id: %w", err)
	}

	f, err := s.repo.FlavorByID(ctx, flavorID)
	if err != nil {
		return nil, fmt.Errorf("flavor by id: %w", err)
	}

	if f.DeletedAt != nil {
		return nil, apierrs.ErrNotFound
	}

	version, err := s.repo.FlavorVersionByID(ctx, versionID)
	if err != nil {
		return nil, fmt.Errorf("flavor version: %w", err)
	}

	return s.filesToUpload(ctx, version)
}
