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
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
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
	"github.com/spacechunks/explorer/internal/tarhelper"
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
) (resource.FlavorVersion, resource.FlavorVersionDiff, error) {
	actorEmail, ok := ctx.Value(contextkey.ActorIDPID).(string)
	if !ok {
		return resource.FlavorVersion{}, resource.FlavorVersionDiff{}, errors.New("actor_id not found in context")
	}

	if err := s.access.AccessAuthorized(
		ctx,
		authz.WithOwnershipRule(actorEmail, authz.FlavorResourceDef(flavorID)),
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

	if err := cleanFileHashPaths(version.FileHashes); err != nil {
		return resource.FlavorVersion{}, resource.FlavorVersionDiff{}, err
	}

	prevVersion, err := s.repo.LatestFlavorVersion(ctx, flavorID)
	if err != nil {
		// super, super, ugly, but as of right now i don't want to refactor
		// (it returns ErrNotFound, if this is the first flavor version)
		if errors.Is(err, apierrs.ErrNotFound) {
			prevVersion.FilesUploaded = true
		} else {
			return resource.FlavorVersion{},
				resource.FlavorVersionDiff{},
				fmt.Errorf("latest flavor version file hashes: %w", err)
		}
	}

	// we do not allow creating a new flavor version when the previous one did not have their
	// files uploaded, because we depend on the uploaded files, when building the image later.
	// this is because we only upload what changed between versions. if the previous changes
	// are not uploaded to s3, the build_image job will fail.
	if !prevVersion.FilesUploaded {
		return resource.FlavorVersion{}, resource.FlavorVersionDiff{}, apierrs.ErrPreviousFilesNotUploaded
	}

	newContentTree, err := file.HashTree(version.FileHashes)
	if err != nil {
		return resource.FlavorVersion{}, resource.FlavorVersionDiff{}, fmt.Errorf("new content tree: %w", err)
	}

	if file.HashTreeRootString(newContentTree) != version.Hash {
		return resource.FlavorVersion{}, resource.FlavorVersionDiff{}, apierrs.ErrHashMismatch
	}

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

	created, err := s.repo.CreateFlavorVersion(ctx, flavorID, version, prevVersion.ID)
	if err != nil {
		return resource.FlavorVersion{}, resource.FlavorVersionDiff{}, fmt.Errorf("create flavor version: %w", err)
	}

	return created, diff, nil
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

	if !version.FilesUploaded {
		exists, err := s.s3Store.ObjectExists(ctx, blob.ChangeSetKey(versionID))
		if err != nil {
			return fmt.Errorf("changeset exists : %w", err)
		}

		if !exists {
			return apierrs.ErrFlavorFilesNotUploaded
		}

		hashes, err := s.computeFileHashes(ctx, versionID)
		if err != nil {
			return fmt.Errorf("compute file hashes: %w", err)
		}

		if err := s.repo.AddFlavorVersionFileHashes(ctx, versionID, hashes); err != nil {
			return fmt.Errorf("add flavor version hashes: %w", err)
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

	c, err := s.repo.ChunkByFlavorID(ctx, flavorID)
	if err != nil {
		return fmt.Errorf("chunk by flavor id: %w", err)
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

func (s *svc) computeFileHashes(ctx context.Context, versionID string) ([]file.Hash, error) {
	dir, err := os.MkdirTemp("", fmt.Sprintf("changeset-%s-*", versionID))
	if err != nil {
		return nil, fmt.Errorf("create tmp dir: %w", err)
	}

	set, err := os.Create(filepath.Join(dir, "changeset.tar.gz"))
	if err != nil {
		return nil, fmt.Errorf("tmp file: %w", err)
	}

	defer set.Close()

	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			s.logger.Error("failed to remove temp dir", "err", err)
		}
	}()

	if err := s.s3Store.WriteTo(ctx, blob.ChangeSetKey(versionID), set); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	if _, err := set.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}

	paths, err := tarhelper.Untar(set, dir)
	if err != nil {
		return nil, fmt.Errorf("untar: %w", err)
	}

	hashes := make([]file.Hash, 0, len(paths))

	for _, p := range paths {
		if err := func() error {
			f, err := os.Open(p)
			if err != nil {
				return fmt.Errorf("open: %w", err)
			}

			defer f.Close()

			hash, err := file.ComputeHashStr(f)
			if err != nil {
				return fmt.Errorf("compute hash: %w", err)
			}

			serverRootPath, err := filepath.Rel(dir, p)
			if err != nil {
				return fmt.Errorf("server root path: %w", err)
			}

			hashes = append(hashes, file.Hash{
				Path: serverRootPath,
				Hash: hash,
			})

			return nil
		}(); err != nil {
			return nil, err
		}
	}

	return hashes, nil
}
