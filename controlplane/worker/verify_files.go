package worker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/riverqueue/river"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/spacechunks/explorer/internal/file"
	"github.com/spacechunks/explorer/internal/resource"
	"github.com/spacechunks/explorer/internal/tarhelper"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

const headCheckConcurrency = 32

// returned when the uploaded tarball itself is the problem. in that case
// retrying won't help, so this is handled in Work and never returned to river.
var errChangeSetInvalid = errors.New("changeset invalid")

type VerifyFilesWorker struct {
	logger *slog.Logger
	river.WorkerDefaults[job.VerifyFiles]
	repo      chunk.Repository
	store     blob.S3Store
	jobClient job.Client
}

func NewVerifyFilesWorker(
	logger *slog.Logger,
	repo chunk.Repository,
	store blob.S3Store,
	jobClient job.Client,
) *VerifyFilesWorker {
	return &VerifyFilesWorker{
		logger:    logger,
		repo:      repo,
		store:     store,
		jobClient: jobClient,
	}
}

func (w *VerifyFilesWorker) Work(ctx context.Context, riverJob *river.Job[job.VerifyFiles]) (ret error) {
	span := trace.SpanFromContext(ctx)
	span.AddLink(trace.Link{
		SpanContext: riverJob.Args.SpanContext.OTel(),
	})

	defer func() {
		if ret == nil {
			return
		}

		w.logger.Error(
			"job failed",
			"chunk_id", riverJob.Args.ChunkID,
			"chunk_name", riverJob.Args.ChunkName,
			"flavor_id", riverJob.Args.FlavorID,
			"flavor_name", riverJob.Args.FlavorName,
			"err", ret,
		)

		// we only want to update the job to failed
		// once we exhausted all attempts.
		if riverJob.Attempt < riverJob.MaxAttempts {
			return
		}

		if err := w.repo.UpdateFlavorVersionBuildStatus(
			ctx,
			riverJob.Args.FlavorVersionID,
			resource.FlavorVersionBuildStatusFilesVerificationFailed,
		); err != nil {
			w.logger.ErrorContext(ctx, "failed to update flavor version build status", "err", err)
		}
	}()

	if err := riverJob.Args.Validate(); err != nil {
		return fmt.Errorf("validate args: %w", err)
	}

	version, err := w.repo.FlavorVersionByID(ctx, riverJob.Args.FlavorVersionID)
	if err != nil {
		return fmt.Errorf("flavor version: %w", err)
	}

	declared := make(map[string]string, len(version.FileHashes))
	for _, fh := range version.FileHashes {
		declared[fh.Path] = fh.Hash
	}

	rootDir := fmt.Sprintf("/tmp/%d", riverJob.ID)
	defer func() {
		if err := os.RemoveAll(rootDir); err != nil {
			w.logger.ErrorContext(ctx, "failed to remove files", "river_job_id", riverJob.ID, "err", err)
		}
	}()

	ingested, err := w.ingestChangeSet(ctx, version.ID, rootDir, declared)
	if err != nil {
		if errors.Is(err, errChangeSetInvalid) {
			return w.fail(ctx, version.ID, err)
		}
		return fmt.Errorf("ingest changeset: %w", err)
	}

	missing, err := w.missingInStore(ctx, version.FileHashes, ingested)
	if err != nil {
		return fmt.Errorf("check blob store: %w", err)
	}

	if len(missing) > 0 {
		paths := make([]string, 0, len(missing))
		for _, fh := range missing {
			paths = append(paths, fh.Path)
		}
		return w.fail(
			ctx,
			version.ID,
			fmt.Errorf("%d files missing in blob store: %s", len(missing), strings.Join(paths, ", ")),
		)
	}

	if err := w.repo.SetFlavorVersionFilesUploaded(ctx, version.ID, true); err != nil {
		return fmt.Errorf("mark files uploaded: %w", err)
	}

	if err := w.repo.ClearFlavorVersionPresignedURLData(ctx, version.ID); err != nil {
		return fmt.Errorf("clear presigned url: %w", err)
	}

	// everything lives in the blob store now, so the tarball can go.
	// not worth failing the job over though.
	if err := w.store.DeleteObject(ctx, blob.ChangeSetKey(version.ID)); err != nil {
		w.logger.WarnContext(ctx, "failed to delete changeset tarball", "flavor_version_id", version.ID, "err", err)
	}

	if err := w.jobClient.InsertJob(
		ctx,
		version.ID,
		string(resource.FlavorVersionBuildStatusBuildImage),
		job.CreateImage{
			FlavorVersionID: riverJob.Args.FlavorVersionID,
			BaseImage:       riverJob.Args.BaseImage,
			OCIRegistry:     riverJob.Args.OCIRegistry,
			SpanContext:     riverJob.Args.SpanContext,
			ChunkID:         riverJob.Args.ChunkID,
			ChunkName:       riverJob.Args.ChunkName,
			FlavorID:        riverJob.Args.FlavorID,
			FlavorName:      riverJob.Args.FlavorName,
		},
	); err != nil {
		return fmt.Errorf("insert create image job: %w", err)
	}

	return nil
}

// returns nil on purpose. the outcome is final and the client has to
// upload again, so there is no point in letting river retry the job.
func (w *VerifyFilesWorker) fail(ctx context.Context, versionID string, reason error) error {
	w.logger.WarnContext(ctx, "files verification failed", "flavor_version_id", versionID, "reason", reason)

	if err := w.repo.UpdateFlavorVersionBuildStatus(
		ctx,
		versionID,
		resource.FlavorVersionBuildStatusFilesVerificationFailed,
	); err != nil {
		return fmt.Errorf("update build status: %w", err)
	}

	if err := w.repo.SetFlavorVersionFilesUploaded(ctx, versionID, false); err != nil {
		return fmt.Errorf("reset files uploaded: %w", err)
	}

	if err := w.repo.ClearFlavorVersionPresignedURLData(ctx, versionID); err != nil {
		return fmt.Errorf("clear presigned url: %w", err)
	}

	return nil
}

func (w *VerifyFilesWorker) ingestChangeSet(
	ctx context.Context,
	versionID string,
	rootDir string,
	declared map[string]string,
) (map[string]struct{}, error) {
	ingested := make(map[string]struct{})

	exists, err := w.store.ObjectExists(ctx, blob.ChangeSetKey(versionID))
	if err != nil {
		return nil, fmt.Errorf("changeset exists: %w", err)
	}

	// no tarball is fine, it just means all files are expected
	// to be in the blob store already.
	if !exists {
		return ingested, nil
	}

	filesDir := filepath.Join(rootDir, "files")
	if err := os.MkdirAll(filesDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("create files dir: %w", err)
	}

	tb, err := os.Create(filepath.Join(rootDir, "changeset.tar.gz"))
	if err != nil {
		return nil, fmt.Errorf("create tarball file: %w", err)
	}
	defer tb.Close()

	if err := w.store.WriteTo(ctx, blob.ChangeSetKey(versionID), tb); err != nil {
		return nil, fmt.Errorf("download tarball: %w", err)
	}

	if _, err := tb.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}

	paths, err := tarhelper.Untar(tb, filesDir)
	if err != nil {
		return nil, fmt.Errorf("%w: untar: %w", errChangeSetInvalid, err)
	}

	objs := make([]blob.Object, 0, len(paths))
	blobs := make([]resource.Blob, 0, len(paths))

	defer func() {
		for _, o := range objs {
			_ = o.Data.Close()
		}
	}()

	// check everything first. we don't want to end up with half of a bad
	// changeset in the blob store.
	for _, p := range paths {
		rel := strings.TrimPrefix(p, filesDir+string(os.PathSeparator))
		rel = filepath.ToSlash(rel)

		want, ok := declared[rel]
		if !ok {
			return nil, fmt.Errorf("%w: file %s is not declared by the flavor version", errChangeSetInvalid, rel)
		}

		obj, err := blob.NewFromFile(p)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", rel, err)
		}
		objs = append(objs, obj)

		got, err := obj.Hash()
		if err != nil {
			return nil, fmt.Errorf("hash %s: %w", rel, err)
		}

		if got != want {
			return nil, fmt.Errorf("%w: hash of %s does not match (want %s, got %s)", errChangeSetInvalid, rel, want, got)
		}

		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", rel, err)
		}

		blobs = append(blobs, resource.Blob{Hash: got, SizeBytes: info.Size()})
	}

	// store will check if there are any duplicates
	if err := w.store.PutBlob(ctx, blob.CASKeyPrefix, objs); err != nil {
		return nil, fmt.Errorf("put blobs: %w", err)
	}

	// only record what is actually in the store, otherwise we are back
	// to the situation where the db claims files exist that don't.
	if err := w.repo.InsertBlobs(ctx, blobs); err != nil {
		return nil, fmt.Errorf("insert blobs: %w", err)
	}

	for _, b := range blobs {
		ingested[b.Hash] = struct{}{}
	}

	return ingested, nil
}

func (w *VerifyFilesWorker) missingInStore(
	ctx context.Context,
	declared []file.Hash,
	ingested map[string]struct{},
) ([]file.Hash, error) {
	toCheck := make([]file.Hash, 0)
	for _, fh := range declared {
		if _, ok := ingested[fh.Hash]; ok {
			continue
		}
		toCheck = append(toCheck, fh)
	}

	results := make([]bool, len(toCheck))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(headCheckConcurrency)

	for i, fh := range toCheck {
		g.Go(func() error {
			exists, err := w.store.ObjectExists(gctx, blob.CASKeyPrefix+"/"+fh.Hash)
			if err != nil {
				return fmt.Errorf("object exists %s: %w", fh.Hash, err)
			}
			results[i] = exists
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	missing := make([]file.Hash, 0)
	stale := make([]string, 0)
	for i, fh := range toCheck {
		if results[i] {
			continue
		}
		missing = append(missing, fh)
		stale = append(stale, fh.Hash)
	}

	// the index said these exist, but they don't. drop the rows so the
	// files show up in GetFilesToUpload again instead of failing forever.
	if err := w.repo.DeleteBlobs(ctx, stale); err != nil {
		return nil, fmt.Errorf("delete stale blob rows: %w", err)
	}

	return missing, nil
}
