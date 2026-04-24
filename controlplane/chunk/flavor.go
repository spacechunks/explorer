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
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/spacechunks/explorer/controlplane/authz"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/contextkey"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/spacechunks/explorer/controlplane/resource"
	"github.com/spacechunks/explorer/internal/file"
	"go.opentelemetry.io/otel/trace"
)

/*
 * service functions
 */

func (s *svc) CreateFlavor(ctx context.Context, chunkID string, flavor resource.Flavor) (resource.Flavor, error) {
	actorID, ok := ctx.Value(contextkey.ActorID).(string)
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
) (resource.FlavorVersion, resource.FlavorVersionDiff, error) {
	actorID, ok := ctx.Value(contextkey.ActorID).(string)
	if !ok {
		return resource.FlavorVersion{}, resource.FlavorVersionDiff{}, errors.New("actor_id not found in context")
	}

	if err := s.access.AccessAuthorized(
		ctx,
		authz.WithOwnershipRule(actorID, authz.FlavorResourceDef(flavorID)),
	); err != nil {
		return resource.FlavorVersion{}, resource.FlavorVersionDiff{}, fmt.Errorf("access: %w", err)
	}

	f, err := s.repo.FlavorByID(ctx, flavorID)
	if err != nil {
		return resource.FlavorVersion{}, resource.FlavorVersionDiff{}, fmt.Errorf("flavor by id: %w", err)
	}

	if f.DeletedAt != nil {
		return resource.FlavorVersion{}, resource.FlavorVersionDiff{}, apierrs.ErrNotFound
	}

	exists, err := s.repo.FlavorVersionExists(ctx, flavorID, version.Version)
	if err != nil {
		return resource.FlavorVersion{},
			resource.FlavorVersionDiff{},
			fmt.Errorf("flavor version exists: %w", err)
	}

	if exists {
		return resource.FlavorVersion{}, resource.FlavorVersionDiff{}, apierrs.ErrFlavorVersionExists
	}

	_, err = s.repo.GetMinecraftVersionByVersion(ctx, version.MinecraftVersion)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return resource.FlavorVersion{},
				resource.FlavorVersionDiff{},
				apierrs.ErrMinecraftVersionNotSupported
		}

		return resource.FlavorVersion{},
			resource.FlavorVersionDiff{},
			fmt.Errorf("minecraft version exists: %w", err)
	}

	prevVersion, err := s.repo.LatestFlavorVersion(ctx, flavorID)
	if err != nil {
		return resource.FlavorVersion{},
			resource.FlavorVersionDiff{},
			fmt.Errorf("latest flavor version file hashes: %w", err)
	}

	newContentTree, err := file.HashTree(version.FileHashes)
	if err != nil {
		return resource.FlavorVersion{}, resource.FlavorVersionDiff{}, fmt.Errorf("new content tree: %w", err)
	}

	if file.HashTreeRootString(newContentTree) != version.Hash {
		return resource.FlavorVersion{}, resource.FlavorVersionDiff{}, apierrs.ErrHashMismatch
	}

	// TODO: clean all received paths using filepath.Clean to avoid
	// possible path traversal techniques. relative paths are not allowed.

	var (
		unchanged = make([]file.Hash, 0)
		changed   = make([]file.Hash, 0)
		added     = make([]file.Hash, 0)
		removed   = make([]file.Hash, 0)
	)

	prevMap := make(map[string]file.Hash, len(prevVersion.FileHashes))
	for _, v := range prevVersion.FileHashes {
		prevMap[v.Path] = v
	}

	uploadedMap := make(map[string]file.Hash, len(version.FileHashes))
	for _, v := range version.FileHashes {
		uploadedMap[v.Path] = v
	}

	for _, prev := range slices.Collect(maps.Values(prevMap)) {
		uploaded, ok := uploadedMap[prev.Path]
		if ok {
			//  did not change, ignore
			if uploaded.Hash == prev.Hash {
				unchanged = append(unchanged, uploaded)
				continue
			}

			changed = append(changed, uploaded)
			continue
		}

		// it does not exist in the uploaded hashes, but was previously present, this means
		// the file has been deleted.
		removed = append(removed, prev)
	}

	for _, uploaded := range slices.Collect(maps.Values(uploadedMap)) {
		if _, ok := prevMap[uploaded.Path]; ok {
			continue
		}

		// the uploaded file was not previously present, this means it is new
		added = append(added, uploaded)
	}

	var (
		diff = resource.FlavorVersionDiff{
			Added:   added,
			Removed: removed,
			Changed: changed,
		}
		sortByPath = func(sl []file.Hash) {
			sort.Slice(sl, func(i, j int) bool {
				return strings.Compare(sl[i].Path, sl[j].Path) < 0
			})
		}
	)

	sortByPath(unchanged)
	sortByPath(changed)
	sortByPath(added)
	sortByPath(removed)

	changes := make([]file.Hash, 0, len(changed)+len(added))
	changes = append(changes, changed...)
	changes = append(changes, added...)
	sortByPath(changes)

	all := make([]file.Hash, 0, len(unchanged)+len(changes))
	all = append(all, changes...)
	all = append(all, unchanged...)

	sortByPath(all)

	changesTree, err := file.HashTree(changes)
	if err != nil {
		return resource.FlavorVersion{}, resource.FlavorVersionDiff{}, fmt.Errorf("changes tree: %w", err)
	}

	version.ChangeHash = file.HashTreeRootString(changesTree)
	version.FileHashes = all

	created, err := s.repo.CreateFlavorVersion(ctx, flavorID, version, prevVersion.ID)
	if err != nil {
		return resource.FlavorVersion{}, resource.FlavorVersionDiff{}, fmt.Errorf("create flavor version: %w", err)
	}

	return created, diff, nil
}

func (s *svc) BuildFlavorVersion(ctx context.Context, versionID string) error {
	actorID, ok := ctx.Value(contextkey.ActorID).(string)
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

	if !version.FilesUploaded {
		exists, err := s.s3Store.ObjectExists(ctx, blob.ChangeSetKey(versionID))
		if err != nil {
			return fmt.Errorf("changeset exists : %w", err)
		}

		if !exists {
			return apierrs.ErrFlavorFilesNotUploaded
		}

		if err := s.repo.MarkFlavorVersionFilesUploaded(ctx, versionID); err != nil {
			return fmt.Errorf("mark files: %w", err)
		}
	}

	// do not fail the request if there is already a job running,
	// or it is already completed, because those states do not
	// indicate that anything is wrong.
	if version.BuildStatus == resource.FlavorVersionBuildStatusBuildCheckpoint ||
		version.BuildStatus == resource.FlavorVersionBuildStatusBuildImage ||
		version.BuildStatus == resource.FlavorVersionBuildStatusCompleted {
		return nil
	}

	span := trace.SpanFromContext(ctx)
	spanCtx := job.SpanContext{
		TraceID: span.SpanContext().TraceID().String(),
		SpanID:  span.SpanContext().SpanID().String(),
	}

	if version.BuildStatus == resource.FlavorVersionBuildStatusBuildCheckpointFailed {
		createCheckpoint := job.CreateCheckpoint{
			FlavorVersionID: versionID,
			BaseImageURL:    fmt.Sprintf("%s/%s:base", s.cfg.Registry, versionID),
			SpanContext:     spanCtx,
		}
		if err := s.jobClient.InsertJob(
			ctx,
			versionID,
			string(resource.FlavorVersionBuildStatusBuildCheckpoint),
			createCheckpoint,
		); err != nil {
			return fmt.Errorf("insert create image job: %w", err)
		}
		return nil
	}

	mcVersion, err := s.repo.GetMinecraftVersionByVersion(ctx, version.MinecraftVersion)
	if err != nil {
		return fmt.Errorf("minecraft version: %w", err)
	}

	if err := s.jobClient.InsertJob(ctx, versionID, string(resource.FlavorVersionBuildStatusBuildImage), job.CreateImage{
		FlavorVersionID: versionID,
		BaseImage:       mcVersion.ImageURL,
		OCIRegistry:     s.cfg.Registry,
		SpanContext:     spanCtx,
	}); err != nil {
		return fmt.Errorf("insert create image job: %w", err)
	}

	return nil
}

func (s *svc) DeleteFlavor(ctx context.Context, id string) error {
	actorID, ok := ctx.Value(contextkey.ActorID).(string)
	if !ok {
		return errors.New("actor_id not found in context")
	}

	if err := s.access.AccessAuthorized(
		ctx,
		authz.WithOwnershipRule(actorID, authz.FlavorResourceDef(id)),
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
