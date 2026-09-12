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
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/spacechunks/explorer/controlplane/authz"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/contextkey"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/spacechunks/explorer/internal/file"
	"github.com/spacechunks/explorer/internal/resource"
	"go.opentelemetry.io/otel/trace"
)

/*
 * service functions
 */

func (s *svc) CreateFlavor(ctx context.Context, chunkID string, flavor resource.Flavor) (resource.Flavor, error) {
	actorID, ok := ctx.Value(contextkey.ActorIDPID).(string)
	if !ok {
		return resource.Flavor{}, errors.New("actor_id not found in context")
	}

	if err := s.access.AccessAuthorized(
		ctx,
		authz.WithOwnershipRule(actorID, authz.ChunkResourceDef(chunkID)),
	); err != nil {
		return resource.Flavor{}, fmt.Errorf("access: %w", err)
	}

	// FIXME: get the flavor by name and then check if the deleted timestamp
	//        is set. if it is, return conflict error or something. returning
	//        ErrFlavorNameExists is a bit in consistent as we return not found
	//        in other endpoints.

	// this will also prevent creation of flavors that are being deleted,
	// because they are still present in the db.
	exists, err := s.repo.FlavorNameExists(ctx, chunkID, flavor.Name)
	if err != nil {
		return resource.Flavor{}, fmt.Errorf("flavor name exists: %w", err)
	}

	if exists {
		return resource.Flavor{}, apierrs.ErrFlavorNameExists
	}

	ret, err := s.repo.CreateFlavor(ctx, chunkID, flavor)
	if err != nil {
		return resource.Flavor{}, fmt.Errorf("create flavor: %w", err)
	}

	return ret, nil
}

func (s *svc) CreateFlavorVersion(
	ctx context.Context,
	flavorID string,
	version resource.FlavorVersion,
) (resource.FlavorVersion, error) {
	actorEmail, ok := ctx.Value(contextkey.ActorIDPID).(string)
	if !ok {
		return resource.FlavorVersion{}, errors.New("actor_id not found in context")
	}

	if err := s.access.AccessAuthorized(
		ctx,
		authz.WithOwnershipRule(actorEmail, authz.FlavorResourceDef(flavorID)),
	); err != nil {
		return resource.FlavorVersion{}, fmt.Errorf("access: %w", err)
	}

	f, err := s.repo.FlavorByID(ctx, flavorID)
	if err != nil {
		return resource.FlavorVersion{}, fmt.Errorf("flavor by id: %w", err)
	}

	if f.DeletedAt != nil {
		return resource.FlavorVersion{}, apierrs.ErrNotFound
	}

	exists, err := s.repo.FlavorVersionExists(ctx, flavorID, version.Version)
	if err != nil {
		return resource.FlavorVersion{}, fmt.Errorf("flavor version exists: %w", err)
	}

	if exists {
		return resource.FlavorVersion{}, apierrs.ErrFlavorVersionExists
	}

	if _, err := s.repo.GetMinecraftVersionByVersion(ctx, version.MinecraftVersion); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return resource.FlavorVersion{}, apierrs.ErrMinecraftVersionNotSupported
		}
		return resource.FlavorVersion{}, fmt.Errorf("minecraft version exists: %w", err)
	}

	if err := cleanFileHashPaths(version.FileHashes); err != nil {
		return resource.FlavorVersion{}, err
	}

	newContentTree, err := file.HashTree(version.FileHashes)
	if err != nil {
		return resource.FlavorVersion{}, fmt.Errorf("new content tree: %w", err)
	}

	if file.HashTreeRootString(newContentTree) != version.Hash {
		return resource.FlavorVersion{}, apierrs.ErrHashMismatch
	}

	prevVersion, err := s.repo.LatestFlavorVersion(ctx, flavorID)
	if err != nil {
		return resource.FlavorVersion{}, fmt.Errorf("latest flavor version: %w", err)
	}

	sort.Slice(version.FileHashes, func(i, j int) bool {
		return strings.Compare(version.FileHashes[i].Path, version.FileHashes[j].Path) < 0
	})

	created, err := s.repo.CreateFlavorVersion(ctx, flavorID, version, prevVersion.ID)
	if err != nil {
		return resource.FlavorVersion{}, fmt.Errorf("create flavor version: %w", err)
	}

	return created, nil
}

func cleanFileHashPaths(hashes []file.Hash) error {
	invalidPaths := make([]apierrs.InvalidPathViolation, 0)
	for i := range hashes {
		cleanPath := filepath.Clean(hashes[i].Path)
		if cleanPath == "." ||
			filepath.IsAbs(cleanPath) ||
			cleanPath == ".." ||
			strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
			invalidPaths = append(invalidPaths, apierrs.InvalidPathViolation{
				Field: fmt.Sprintf("version.file_hashes[%d].path", i),
				Path:  hashes[i].Path,
			})
			continue
		}

		hashes[i].Path = filepath.ToSlash(cleanPath)
	}

	if len(invalidPaths) > 0 {
		return apierrs.InvalidPath(invalidPaths...)
	}

	return nil
}

func (s *svc) BuildFlavorVersion(ctx context.Context, versionID string) error {
	actorID, ok := ctx.Value(contextkey.ActorIDPID).(string)
	if !ok {
		return errors.New("actor_id not found in context")
	}

	if err := s.access.AccessAuthorized(
		ctx,
		authz.WithOwnershipRule(actorID, authz.FlavorVersionResourceDef(versionID)),
	); err != nil {
		return fmt.Errorf("access: %w", err)
	}

	flavorID, err := s.repo.FlavorIDByFlavorVersionID(ctx, versionID)
	if err != nil {
		return fmt.Errorf("get flavor id: %w", err)
	}

	f, err := s.repo.FlavorByID(ctx, flavorID)
	if err != nil {
		return fmt.Errorf("flavor by id: %w", err)
	}

	if f.DeletedAt != nil {
		return apierrs.ErrNotFound
	}

	version, err := s.repo.FlavorVersionByID(ctx, versionID)
	if err != nil {
		return fmt.Errorf("flavor version: %w", err)
	}

	// do not fail the request if there is already a job running,
	// or it is already completed, because those states do not
	// indicate that anything is wrong.
	if version.BuildStatus == resource.FlavorVersionBuildStatusFilesVerification ||
		version.BuildStatus == resource.FlavorVersionBuildStatusBuildCheckpoint ||
		version.BuildStatus == resource.FlavorVersionBuildStatusBuildImage ||
		version.BuildStatus == resource.FlavorVersionBuildStatusCompleted {
		return nil
	}

	span := trace.SpanFromContext(ctx)
	spanCtx := job.SpanContext{
		TraceID: span.SpanContext().TraceID().String(),
		SpanID:  span.SpanContext().SpanID().String(),
	}

	c, err := s.repo.ChunkByFlavorID(ctx, flavorID)
	if err != nil {
		return fmt.Errorf("chunk by flavor id: %w", err)
	}

	mcVersion, err := s.repo.GetMinecraftVersionByVersion(ctx, version.MinecraftVersion)
	if err != nil {
		return fmt.Errorf("minecraft version: %w", err)
	}

	missing, err := s.filesToUpload(ctx, version)
	if err != nil {
		return fmt.Errorf("files to upload: %w", err)
	}

	// the blob index is the single source of truth. versions that were marked as
	// uploaded by the previous flow, but whose files never reached the blob store,
	// are routed through verification as well.
	if !version.FilesUploaded || len(missing) > 0 {
		if len(missing) > 0 {
			exists, err := s.s3Store.ObjectExists(ctx, blob.ChangeSetKey(versionID))
			if err != nil {
				return fmt.Errorf("changeset exists: %w", err)
			}

			if !exists {
				return apierrs.ErrFlavorFilesNotUploaded
			}
		}

		if err := s.jobClient.InsertJob(
			ctx,
			versionID,
			string(resource.FlavorVersionBuildStatusFilesVerification),
			job.VerifyFiles{
				FlavorVersionID: versionID,
				BaseImage:       mcVersion.ImageURL,
				OCIRegistry:     s.cfg.Registry,
				SpanContext:     spanCtx,
				ChunkID:         c.ID,
				ChunkName:       c.Name,
				FlavorID:        f.ID,
				FlavorName:      f.Name,
			},
		); err != nil {
			return fmt.Errorf("insert verify files job: %w", err)
		}
		return nil
	}

	if version.BuildStatus == resource.FlavorVersionBuildStatusBuildCheckpointFailed {
		createCheckpoint := job.CreateCheckpoint{
			FlavorVersionID: versionID,
			BaseImageURL:    fmt.Sprintf("%s/%s:base", s.cfg.Registry, versionID),
			SpanContext:     spanCtx,
			ChunkID:         c.ID,
			ChunkName:       c.Name,
			FlavorID:        f.ID,
			FlavorName:      f.Name,
		}
		if err := s.jobClient.InsertJob(
			ctx,
			versionID,
			string(resource.FlavorVersionBuildStatusBuildCheckpoint),
			createCheckpoint,
		); err != nil {
			return fmt.Errorf("insert create checkpoint job: %w", err)
		}
		return nil
	}

	if err := s.jobClient.InsertJob(ctx, versionID, string(resource.FlavorVersionBuildStatusBuildImage), job.CreateImage{
		FlavorVersionID: versionID,
		BaseImage:       mcVersion.ImageURL,
		OCIRegistry:     s.cfg.Registry,
		SpanContext:     spanCtx,
		ChunkID:         c.ID,
		ChunkName:       c.Name,
		FlavorID:        f.ID,
		FlavorName:      f.Name,
	}); err != nil {
		return fmt.Errorf("insert create image job: %w", err)
	}

	return nil
}

func (s *svc) DeleteFlavor(ctx context.Context, id string) error {
	actorEmail, ok := ctx.Value(contextkey.ActorIDPID).(string)
	if !ok {
		return errors.New("actor_id not found in context")
	}

	if err := s.access.AccessAuthorized(
		ctx,
		authz.WithOwnershipRule(actorEmail, authz.FlavorResourceDef(id)),
	); err != nil {
		return fmt.Errorf("access: %w", err)
	}

	f, err := s.repo.FlavorByID(ctx, id)
	if err != nil {
		return fmt.Errorf("flavor by id: %w", err)
	}

	if f.DeletedAt != nil {
		return nil
	}

	if err := s.repo.MarkFlavorDeleted(ctx, id); err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	return nil
}

func (s *svc) GetFlavor(ctx context.Context, id string) (resource.Flavor, error) {
	_, ok := ctx.Value(contextkey.ActorIDPID).(string)
	if !ok {
		return resource.Flavor{}, errors.New("actor_id not found in context")
	}

	f, err := s.repo.FlavorByID(ctx, id)
	if err != nil {
		return resource.Flavor{}, fmt.Errorf("flavor by id: %w", err)
	}
	if f.DeletedAt != nil {
		return resource.Flavor{}, apierrs.ErrNotFound
	}

	return f, nil
}
