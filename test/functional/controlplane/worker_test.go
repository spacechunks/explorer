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

package controlplane

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	ociv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/spacechunks/explorer/internal/file"
	"github.com/spacechunks/explorer/internal/image"
	imgtestdata "github.com/spacechunks/explorer/internal/image/testdata"
	"github.com/spacechunks/explorer/internal/ptr"
	"github.com/spacechunks/explorer/internal/resource"
	"github.com/spacechunks/explorer/internal/tarhelper"
	"github.com/spacechunks/explorer/test"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/spacechunks/explorer/test/functional/controlplane/testdata"
	"github.com/stretchr/testify/require"
)

var auth = remote.WithAuth(&image.Auth{
	Username: fixture.OCIRegsitryUser,
	Password: fixture.OCIRegistryPass,
})

func imageWorkerSetup(
	t *testing.T,
	ctx context.Context,
	c *resource.Chunk,
	auth remote.Option,
	changeSet []byte,
) (*fixture.Postgres, fixture.FakeS3, string, name.Reference) {
	var (
		pg               = fixture.NewPostgres()
		registryEndpoint = fixture.RunRegistry(t)
		fakes3           = fixture.RunFakeS3(t)
	)

	pg.Run(t, ctx)
	pg.CreateRiverClient(t)
	pg.InsertMinecraftVersion(t)
	pg.CreateChunk(t, c, fixture.CreateOptionsAll)

	pusher, err := remote.NewPusher(auth)
	require.NoError(t, err)

	baseImgRef, err := name.ParseReference(fmt.Sprintf("%s/%s", registryEndpoint, fixture.BaseImage))
	require.NoError(t, err)

	err = pusher.Push(ctx, baseImgRef, imgtestdata.Image(t))
	require.NoError(t, err)

	if changeSet != nil {
		fakes3.UploadObject(t, blob.ChangeSetKey(c.Flavors[0].Versions[0].ID), changeSet)
	}

	return pg, fakes3, registryEndpoint, baseImgRef
}

func insertVerifyFilesJob(
	ctx context.Context,
	pg *fixture.Postgres,
	flavorVersionID string,
	baseImgRef name.Reference,
	endpoint string,
) error {
	return pg.DB.InsertJob(
		ctx,
		flavorVersionID,
		string(resource.FlavorVersionBuildStatusFilesVerification),
		job.VerifyFiles{
			FlavorVersionID: flavorVersionID,
			BaseImage:       baseImgRef.String(),
			OCIRegistry:     endpoint,
		},
	)
}

// puts every file of the serverdata testdata into the blob store and returns their hashes.
func seedBlobStore(t *testing.T, ctx context.Context) []string {
	var (
		store  = blob.NewS3Store(fixture.Bucket, fixture.NewS3Client(t, ctx), nil)
		objs   = make([]blob.Object, 0)
		hashes = make([]string, 0)
	)

	err := filepath.WalkDir("./testdata/serverdata", func(path string, d fs.DirEntry, _ error) error {
		if d.IsDir() {
			return nil
		}

		obj, err := blob.NewFromFile(path)
		require.NoError(t, err)

		h, err := obj.Hash()
		require.NoError(t, err)

		objs = append(objs, obj)
		hashes = append(hashes, h)
		return nil
	})
	require.NoError(t, err)

	err = store.PutBlob(ctx, blob.CASKeyPrefix, objs)
	require.NoError(t, err)

	return hashes
}

func waitForBuildStatus(
	t *testing.T,
	ctx context.Context,
	pg *fixture.Postgres,
	flavorVersionID string,
	status resource.FlavorVersionBuildStatus,
) resource.FlavorVersion {
	var (
		timeoutCtx, cancel = context.WithTimeout(ctx, 20*time.Second)
		ticker             = time.NewTicker(200 * time.Millisecond)
	)

	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			t.Fatalf("timeout reached waiting for build status %s", status)
			return resource.FlavorVersion{}
		case <-ticker.C:
			version, err := pg.DB.FlavorVersionByID(ctx, flavorVersionID)
			require.NoError(t, err)

			if version.BuildStatus == status {
				return version
			}
		}
	}
}

func computeTestFileHash(t *testing.T) string {
	f, err := os.Open("./testdata/testfile1.txt")
	require.NoError(t, err)

	defer f.Close()

	hash, err := file.ComputeHashStr(f)
	require.NoError(t, err)

	return hash
}

// returns the distinct hashes, because files with the same
// content only show up once in the blob store.
func sortedHashes(fileHashes []file.Hash) []string {
	seen := make(map[string]struct{}, len(fileHashes))
	hashes := make([]string, 0, len(fileHashes))
	for _, fh := range fileHashes {
		if _, ok := seen[fh.Hash]; ok {
			continue
		}
		seen[fh.Hash] = struct{}{}
		hashes = append(hashes, fh.Hash)
	}
	sort.Strings(hashes)
	return hashes
}

func TestImageWorkerCreatesImageFromBlobStore(t *testing.T) {
	var (
		ctx        = context.Background()
		fileHashes = testdata.ComputeFileHashes(t, "./testdata/serverdata")
		c          = ptr.Pointer(fixture.Chunk(func(tmp *resource.Chunk) {
			tmp.Flavors[0].Versions[0].FileHashes = fileHashes
			tmp.Flavors[0].Versions[0].FilesUploaded = true
		}))
	)

	pg, _, endpoint, baseImgRef := imageWorkerSetup(t, ctx, c, auth, nil)

	flavorVersionID := c.Flavors[0].Versions[0].ID

	seedBlobStore(t, ctx)

	err := pg.DB.InsertJob(ctx, flavorVersionID, string(resource.FlavorVersionBuildStatusBuildImage), job.CreateImage{
		FlavorVersionID: flavorVersionID,
		BaseImage:       baseImgRef.String(),
		OCIRegistry:     endpoint,
	})
	require.NoError(t, err)

	checkImage(t, ctx, auth, endpoint, flavorVersionID, fileHashes)
}

func TestImageWorkerRefusesUnverifiedFiles(t *testing.T) {
	var (
		ctx        = context.Background()
		fileHashes = testdata.ComputeFileHashes(t, "./testdata/serverdata")
		c          = ptr.Pointer(fixture.Chunk(func(tmp *resource.Chunk) {
			tmp.Flavors[0].Versions[0].FileHashes = fileHashes
			tmp.Flavors[0].Versions[0].FilesUploaded = false
		}))
	)

	pg, _, endpoint, baseImgRef := imageWorkerSetup(t, ctx, c, auth, nil)

	flavorVersionID := c.Flavors[0].Versions[0].ID

	seedBlobStore(t, ctx)

	err := pg.DB.InsertJob(ctx, flavorVersionID, string(resource.FlavorVersionBuildStatusBuildImage), job.CreateImage{
		FlavorVersionID: flavorVersionID,
		BaseImage:       baseImgRef.String(),
		OCIRegistry:     endpoint,
	})
	require.NoError(t, err)

	var (
		timeoutCtx, cancel = context.WithTimeout(ctx, 20*time.Second)
		ticker             = time.NewTicker(200 * time.Millisecond)
	)

	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			t.Fatal("timeout reached")
			return
		case <-ticker.C:
			var errs string
			err := pg.Pool.
				QueryRow(ctx, `SELECT COALESCE(errors::text, '') FROM river_job WHERE kind = $1`, job.CreateImage{}.Kind()).
				Scan(&errs)
			require.NoError(t, err)

			if !strings.Contains(errs, "have not been verified") {
				continue
			}

			return
		}
	}
}

func TestVerifyFilesWorkerIngestsChangeSetAndBuildsImage(t *testing.T) {
	var (
		ctx        = context.Background()
		fileHashes = testdata.ComputeFileHashes(t, "./testdata/serverdata")
		c          = ptr.Pointer(fixture.Chunk(func(tmp *resource.Chunk) {
			tmp.Flavors[0].Versions[0].FileHashes = fileHashes
		}))
	)

	pg, fakes3, endpoint, baseImgRef := imageWorkerSetup(t, ctx, c, auth, testdata.FullChangeSetFile)

	flavorVersionID := c.Flavors[0].Versions[0].ID

	err := insertVerifyFilesJob(ctx, pg, flavorVersionID, baseImgRef, endpoint)
	require.NoError(t, err)

	// the image only gets built if the verify worker chained the image job
	checkImage(t, ctx, auth, endpoint, flavorVersionID, fileHashes)

	version, err := pg.DB.FlavorVersionByID(ctx, flavorVersionID)
	require.NoError(t, err)
	require.True(t, version.FilesUploaded)
	require.Nil(t, version.PresignedURLExpiryDate)

	require.Equal(t, sortedHashes(fileHashes), pg.BlobHashes(t))

	for _, fh := range fileHashes {
		fakes3.RequireObjectExists(t, blob.CASKeyPrefix+"/"+fh.Hash)
	}

	require.False(t, fakes3.ObjectExists(t, blob.ChangeSetKey(flavorVersionID)), "tarball should be gone")
}

func TestVerifyFilesWorkerFailsWhenFilesAreMissing(t *testing.T) {
	var (
		ctx          = context.Background()
		fileHashes   = testdata.ComputeFileHashes(t, "./testdata/serverdata")
		testFileHash = computeTestFileHash(t)
		c            = ptr.Pointer(fixture.Chunk(func(tmp *resource.Chunk) {
			tmp.Flavors[0].Versions[0].FileHashes = append(fileHashes, file.Hash{
				Path: "testfile1.txt",
				Hash: testFileHash,
			})
		}))
	)

	// the changeset only contains testfile1.txt, everything else is missing
	pg, fakes3, endpoint, baseImgRef := imageWorkerSetup(t, ctx, c, auth, testdata.AddTestFileChangeSet)

	flavorVersionID := c.Flavors[0].Versions[0].ID

	err := pg.DB.UpdateFlavorVersionPresignedURLData(
		ctx,
		flavorVersionID,
		time.Now().Add(1*time.Hour),
		"http://example.com",
	)
	require.NoError(t, err)

	err = insertVerifyFilesJob(ctx, pg, flavorVersionID, baseImgRef, endpoint)
	require.NoError(t, err)

	version := waitForBuildStatus(t, ctx, pg, flavorVersionID, resource.FlavorVersionBuildStatusFilesVerificationFailed)

	require.False(t, version.FilesUploaded)
	require.Nil(t, version.PresignedURLExpiryDate, "presigned url should be cleared")

	// what was in the tarball is fine and has been ingested, the rest is missing
	require.Equal(t, []string{testFileHash}, pg.BlobHashes(t))
	fakes3.RequireObjectExists(t, blob.CASKeyPrefix+"/"+testFileHash)

	require.True(t, fakes3.ObjectExists(t, blob.ChangeSetKey(flavorVersionID)), "tarball should be kept")
}

func TestVerifyFilesWorkerRejectsUndeclaredFiles(t *testing.T) {
	var (
		ctx          = context.Background()
		fileHashes   = testdata.ComputeFileHashes(t, "./testdata/serverdata")
		testFileHash = computeTestFileHash(t)
		c            = ptr.Pointer(fixture.Chunk(func(tmp *resource.Chunk) {
			tmp.Flavors[0].Versions[0].FileHashes = fileHashes
		}))
	)

	// testfile1.txt is not part of the declared files
	pg, fakes3, endpoint, baseImgRef := imageWorkerSetup(t, ctx, c, auth, testdata.AddTestFileChangeSet)

	flavorVersionID := c.Flavors[0].Versions[0].ID

	err := insertVerifyFilesJob(ctx, pg, flavorVersionID, baseImgRef, endpoint)
	require.NoError(t, err)

	version := waitForBuildStatus(t, ctx, pg, flavorVersionID, resource.FlavorVersionBuildStatusFilesVerificationFailed)

	require.False(t, version.FilesUploaded)

	// nothing of a bad changeset must end up in the blob store
	require.Empty(t, pg.BlobHashes(t))
	require.False(t, fakes3.ObjectExists(t, blob.CASKeyPrefix+"/"+testFileHash))
}

func TestVerifyFilesWorkerSucceedsWithoutChangeSet(t *testing.T) {
	var (
		ctx        = context.Background()
		fileHashes = testdata.ComputeFileHashes(t, "./testdata/serverdata")
		c          = ptr.Pointer(fixture.Chunk(func(tmp *resource.Chunk) {
			tmp.Flavors[0].Versions[0].FileHashes = fileHashes
		}))
	)

	pg, _, endpoint, baseImgRef := imageWorkerSetup(t, ctx, c, auth, nil)

	flavorVersionID := c.Flavors[0].Versions[0].ID

	// everything is already in the blob store, e.g. because a previous
	// version consisted of the exact same files.
	hashes := seedBlobStore(t, ctx)
	pg.InsertBlobs(t, hashes...)

	err := insertVerifyFilesJob(ctx, pg, flavorVersionID, baseImgRef, endpoint)
	require.NoError(t, err)

	checkImage(t, ctx, auth, endpoint, flavorVersionID, fileHashes)

	version, err := pg.DB.FlavorVersionByID(ctx, flavorVersionID)
	require.NoError(t, err)
	require.True(t, version.FilesUploaded)
}

func TestVerifyFilesWorkerRemovesStaleBlobRows(t *testing.T) {
	var (
		ctx        = context.Background()
		fileHashes = testdata.ComputeFileHashes(t, "./testdata/serverdata")
		c          = ptr.Pointer(fixture.Chunk(func(tmp *resource.Chunk) {
			tmp.Flavors[0].Versions[0].FileHashes = fileHashes
		}))
	)

	pg, _, endpoint, baseImgRef := imageWorkerSetup(t, ctx, c, auth, nil)

	flavorVersionID := c.Flavors[0].Versions[0].ID

	// the index claims the files exist, but s3 has nothing
	pg.InsertBlobs(t, sortedHashes(fileHashes)...)

	err := insertVerifyFilesJob(ctx, pg, flavorVersionID, baseImgRef, endpoint)
	require.NoError(t, err)

	version := waitForBuildStatus(t, ctx, pg, flavorVersionID, resource.FlavorVersionBuildStatusFilesVerificationFailed)

	require.False(t, version.FilesUploaded)
	require.Empty(t, pg.BlobHashes(t), "stale rows should have been removed")
}

func TestResourcePackWorkerRunsSuccessfully(t *testing.T) {
	var (
		ctx             = context.Background()
		itemTemplate    = `{"model":{"type":"minecraft:model","model":"spacechunks:item/spc/test/{chunk_id}"}}`
		modelTemplate   = `{"parent":"spacechunks:item/explorer/chunk_viewer/flat_ui_element","textures":{"layer0":"spacechunks:item/spc/test/{chunk_id}"}}` //nolint:lll
		templatePack, _ = test.CreateResourcePackZip(t, map[string]string{
			"assets/spc/items/test/_template.json":       itemTemplate,
			"assets/spc/models/item/test/_template.json": modelTemplate,
		})
		c1 = fixture.Chunk(func(tmp *resource.Chunk) {
			tmp.ID = test.NewUUIDv7(t)
			tmp.Thumbnail = resource.Thumbnail{
				Hash: "id1",
			}
		})
		c2 = fixture.Chunk(func(tmp *resource.Chunk) {
			tmp.ID = test.NewUUIDv7(t)
			tmp.Thumbnail = resource.Thumbnail{
				Hash: "id2",
			}
		})

		fakes3 = fixture.RunFakeS3(t)
		cp     = fixture.NewControlPlane(t)
	)

	fakes3.UploadObject(t, fixture.ResourcePackTemplateKey, templatePack.Bytes())
	fakes3.UploadObject(t, blob.CASKeyPrefix+"/"+"id1", []byte(""))
	fakes3.UploadObject(t, blob.CASKeyPrefix+"/"+"id2", []byte(""))

	cp.Run(t)

	cp.Postgres.CreateChunk(t, &c1, fixture.CreateOptionsAll)
	cp.Postgres.CreateChunk(t, &c2, fixture.CreateOptionsAll)

	// wait a second here, so the resource pack worker actually picked up
	// all the chunks. if we don't wait we could run into the problem that
	// no chunk has been processed, because the cp.Run() will cause the job
	// to directly start. sometimes CreateChunk is fast enough and the chunks
	// are already in the database, sometimes not.
	time.Sleep(1 * time.Second)

	_, expectedHash := test.CreateResourcePackZip(t, map[string]string{
		// items
		"assets/spc/items/test/_template.json":              itemTemplate,
		fmt.Sprintf("assets/spc/items/test/%s.json", c1.ID): strings.ReplaceAll(itemTemplate, "{chunk_id}", c1.ID),
		fmt.Sprintf("assets/spc/items/test/%s.json", c2.ID): strings.ReplaceAll(itemTemplate, "{chunk_id}", c2.ID),

		// models
		"assets/spc/models/item/test/_template.json":              modelTemplate,
		fmt.Sprintf("assets/spc/models/item/test/%s.json", c1.ID): strings.ReplaceAll(modelTemplate, "{chunk_id}", c1.ID),
		fmt.Sprintf("assets/spc/models/item/test/%s.json", c2.ID): strings.ReplaceAll(modelTemplate, "{chunk_id}", c2.ID),

		// textures
		fmt.Sprintf("assets/spc/textures/item/test/%s.png", c1.ID): "",
		fmt.Sprintf("assets/spc/textures/item/test/%s.png", c2.ID): "",
	})

	var (
		timeoutCtx, cancel = context.WithTimeout(ctx, 20*time.Second)
		ticker             = time.NewTicker(1 * time.Second)
	)

	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			t.Fatal("timout reached")
			return
		case <-ticker.C:
			key := "explorer/latest.zip"
			if !fakes3.ObjectExists(t, key) {
				t.Log("waiting for object", key)
				continue
			}
			data, metadata := fakes3.GetObject(t, key)

			actualHash := fmt.Sprintf("%x", sha1.Sum(data))

			require.Equal(t, expectedHash, actualHash, "sha1 of binary doesnt match")
			require.Equal(t, expectedHash, metadata["sha1"], "sha1 in metadata does not match")

			return
		}
	}
}

func checkImage(
	t *testing.T,
	ctx context.Context,
	auth remote.Option,
	endpoint string,
	flavorVersionID string,
	fileHashes []file.Hash,
) {
	var (
		timeoutCtx, cancel = context.WithTimeout(ctx, 20*time.Second)
		ticker             = time.NewTicker(1 * time.Second)
	)

	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			t.Fatal("timout reached")
			return
		case <-ticker.C:
			reg, err := name.NewRegistry(endpoint)
			require.NoError(t, err)

			p, err := remote.NewPuller(auth)
			require.NoError(t, err)

			cat, err := p.Catalog(ctx, reg)
			require.NoError(t, err)

			if !slices.Contains(cat, flavorVersionID) {
				continue
			}

			ref, err := name.ParseReference(fmt.Sprintf("%s/%s:base", endpoint, flavorVersionID))
			require.NoError(t, err)

			_, err = p.Head(ctx, ref)
			if err != nil {
				fmt.Println("head image:", err)
				continue
			}

			img, err := remote.Image(ref, auth)
			require.NoError(t, err)

			want := make([]string, 0)
			for _, fh := range fileHashes {
				want = append(want, "opt/paper/"+fh.Path)
			}

			checkLayers(t, img, want)
			return
		}
	}
}

func checkLayers(t *testing.T, img ociv1.Image, want []string) {
	layers, err := img.Layers()
	require.NoError(t, err)

	tmpDir := t.TempDir()
	errors := make([]string, 0)

	sort.Slice(want, func(i, j int) bool {
		return strings.Compare(want[i], want[j]) < 0
	})

	for _, l := range layers {
		rc, err := l.Compressed()
		require.NoError(t, err)

		defer rc.Close()

		h, err := l.Digest()
		require.NoError(t, err)

		dest := filepath.Join(tmpDir, h.Hex)

		paths, err := tarhelper.Untar(rc, dest)
		require.NoError(t, err)

		got := make([]string, 0)
		for _, p := range paths {
			got = append(got, strings.ReplaceAll(p, dest+"/", ""))
		}

		sort.Slice(got, func(i, j int) bool {
			return strings.Compare(got[i], got[j]) < 0
		})

		if d := cmp.Diff(want, got); d == "" {
			return
		} else {
			errors = append(errors, d)
		}
	}

	for _, err := range errors {
		fmt.Println(err)
		fmt.Println("=========")
	}
}
