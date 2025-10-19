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
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/spacechunks/explorer/internal/file"
	"github.com/spacechunks/explorer/internal/image"
	imgtestdata "github.com/spacechunks/explorer/internal/image/testdata"
	"github.com/spacechunks/explorer/internal/ptr"
	"github.com/spacechunks/explorer/internal/tarhelper"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/spacechunks/explorer/test/functional/controlplane/testdata"
	"github.com/stretchr/testify/require"
)

var auth = remote.WithAuth(&image.Auth{
	Username: fixture.OCIRegsitryUser,
	Password: fixture.OCIRegistryPass,
})

func setup(
	t *testing.T,
	ctx context.Context,
	c *chunk.Chunk,
	auth remote.Option,
	changeSet []byte,
) (*fixture.Postgres, string, name.Reference) {
	var (
		pg       = fixture.NewPostgres()
		endpoint = fixture.RunRegistry(t)
		fakes3   = fixture.RunFakeS3(t)
	)

	pg.Run(t, ctx)
	pg.CreateRiverClient(t)
	pg.CreateChunk(t, c, fixture.CreateOptionsAll)

	pusher, err := remote.NewPusher(auth)
	require.NoError(t, err)

	baseImgRef, err := name.ParseReference(fmt.Sprintf("%s/%s", endpoint, fixture.BaseImage))
	require.NoError(t, err)

	err = pusher.Push(ctx, baseImgRef, imgtestdata.Image(t))
	require.NoError(t, err)

	fakes3.UploadObject(t, blob.ChangeSetKey(c.Flavors[0].Versions[0].ID), changeSet)

	return pg, endpoint, baseImgRef
}

func TestImageWorkerCreatesImageWithNoMissingFiles(t *testing.T) {
	var (
		ctx        = context.Background()
		fileHashes = testdata.ComputeFileHashes(t, "./testdata/serverdata")
		c          = ptr.Pointer(fixture.Chunk(func(tmp *chunk.Chunk) {
			tmp.Flavors[0].Versions[0].FileHashes = fileHashes
		}))
	)

	pg, endpoint, baseImgRef := setup(t, ctx, c, auth, testdata.FullChangeSetFile)

	flavorVersionID := c.Flavors[0].Versions[0].ID

	err := pg.DB.InsertJob(ctx, flavorVersionID, string(chunk.BuildStatusBuildImage), job.CreateImage{
		FlavorVersionID: flavorVersionID,
		BaseImage:       baseImgRef.String(),
		OCIRegistry:     endpoint,
	})
	require.NoError(t, err)

	checkImage(t, ctx, auth, endpoint, flavorVersionID, fileHashes)
}

func TestImageWorkerCreatesImageWithMissingFilesDownloadedFromBlobStore(t *testing.T) {
	var (
		ctx        = context.Background()
		fileHashes = testdata.ComputeFileHashes(t, "./testdata/serverdata")
		c          = ptr.Pointer(fixture.Chunk(func(tmp *chunk.Chunk) {
			f, err := os.Open("./testdata/testfile1.txt")
			require.NoError(t, err)

			defer f.Close()

			hash, err := file.ComputeHashStr(f)
			require.NoError(t, err)

			fileHashes = append(fileHashes, file.Hash{
				Path: "testfile1.txt",
				Hash: hash,
			})
			tmp.Flavors[0].Versions[0].FileHashes = fileHashes
		}))
		auth = remote.WithAuth(&image.Auth{
			Username: fixture.OCIRegsitryUser,
			Password: fixture.OCIRegistryPass,
		})
	)

	pg, endpoint, baseImgRef := setup(t, ctx, c, auth, testdata.AddTestFileChangeSet)

	var (
		flavorVersionID = c.Flavors[0].Versions[0].ID
		store           = blob.NewS3Store(fixture.Bucket, fixture.NewS3Client(t, ctx), nil)
		objs            = make([]blob.Object, 0)
	)

	err := filepath.WalkDir("./testdata/serverdata", func(path string, d fs.DirEntry, _ error) error {
		if d.IsDir() {
			return nil
		}

		obj, err := blob.NewFromFile(path)
		require.NoError(t, err)

		objs = append(objs, obj)
		return nil
	})
	require.NoError(t, err)

	err = store.Put(ctx, blob.CASKeyPrefix, objs)
	require.NoError(t, err)

	err = pg.DB.InsertJob(ctx, flavorVersionID, string(chunk.BuildStatusBuildImage), job.CreateImage{
		FlavorVersionID: flavorVersionID,
		BaseImage:       baseImgRef.String(),
		OCIRegistry:     endpoint,
	})
	require.NoError(t, err)

	checkImage(t, ctx, auth, endpoint, flavorVersionID, fileHashes)
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
