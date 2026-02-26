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

package worker

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
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
)

const (
	outDir           = "out"
	basePackFileName = "pack_template.zip"
	uploadKey        = "explorer/latest.zip"
)

type CreateResourcePackWorkerConfig struct {
	WorkingDir        string
	PackTemplateKey   string
	ItemTemplatePath  string
	ModelTemplatePath string
	ModelDir          string
	ItemDir           string
	TextureDir        string
}

type CreateResourcePackWorker struct {
	river.WorkerDefaults[job.CreateResourcePack]

	logger  *slog.Logger
	s3store blob.S3Store
	repo    chunk.Repository
	cfg     CreateResourcePackWorkerConfig
}

func NewCreateResourcePackWorker(
	logger *slog.Logger,
	s3store blob.S3Store,
	repo chunk.Repository,
	cfg CreateResourcePackWorkerConfig,
) *CreateResourcePackWorker {
	return &CreateResourcePackWorker{
		logger:  logger,
		s3store: s3store,
		repo:    repo,
		cfg:     cfg,
	}
}

func (w *CreateResourcePackWorker) Work(ctx context.Context, _ *river.Job[job.CreateResourcePack]) error {
	// FIXME: at some point we only want to upload the pack if it has actually changed.

	dir, err := os.OpenRoot(w.cfg.WorkingDir)
	if err != nil {
		return fmt.Errorf("open build dir: %w", err)
	}

	defer func() {
		dir.RemoveAll(w.cfg.WorkingDir)
		dir.Close()
	}()

	if err := w.fetchAndUnzipBasePackTo(ctx, dir, outDir); err != nil {
		return fmt.Errorf("fetch and unzip base pack: %w", err)
	}

	mergedDirPath := filepath.Join(dir.Name(), outDir)

	w.logger.InfoContext(ctx, "merge dir", "path", mergedDirPath)

	itemTemplate, err := os.ReadFile(filepath.Join(mergedDirPath, w.cfg.ItemTemplatePath))
	if err != nil {
		return fmt.Errorf("read item template: %w", err)
	}

	w.logger.InfoContext(ctx, "item template", "template", string(itemTemplate))

	modelTemplate, err := os.ReadFile(filepath.Join(mergedDirPath, w.cfg.ModelTemplatePath))
	if err != nil {
		return fmt.Errorf("read model template: %w", err)
	}

	w.logger.InfoContext(ctx, "model template", "template", string(modelTemplate))

	// for now, it is fine to simply fetch all thumbnails there are at once.
	// there are smarter ways to do this like, only fetching the ones that
	// have changed, for example. right now, for the foreseeable future this
	// will not really be a concern, so we went with the simplest solution
	// possible. we can focus on this later once this really becomes a problem.
	hashes, err := w.repo.AllChunkThumbnailHashes(ctx)
	if err != nil {
		return fmt.Errorf("thumbnail hashes: %w", err)
	}

	w.logger.InfoContext(ctx, "found thumbnail hashes", "count", len(hashes))

	for id, h := range hashes {
		var (
			item            = strings.ReplaceAll(string(itemTemplate), "{chunk_id}", id)
			model           = strings.ReplaceAll(string(modelTemplate), "{chunk_id}", id)
			jsonFileName    = fmt.Sprintf("%s.json", id)
			textureFileName = fmt.Sprintf("%s.png", id)
			itemFilePath    = filepath.Join(outDir, w.cfg.ItemDir, jsonFileName)
			modelFilePath   = filepath.Join(outDir, w.cfg.ModelDir, jsonFileName)
			textureFilePath = filepath.Join(outDir, w.cfg.TextureDir, textureFileName)
		)

		// for debug purposes
		var (
			itemComp  = &bytes.Buffer{}
			modelComp = &bytes.Buffer{}
		)

		if err := json.Compact(itemComp, []byte(item)); err != nil {
			return fmt.Errorf("compact item: %w", err)
		}

		if err := json.Compact(modelComp, []byte(item)); err != nil {
			return fmt.Errorf("compact item: %w", err)
		}

		w.logger.InfoContext(
			ctx,
			"adding thumbnail",
			"chunk_id", id,
			"thumbnail_hash", h,
			"model_file_path", modelFilePath,
			"item_file_path", itemFilePath,
			"texture_file_path", textureFilePath,
			"item", itemComp.String(),
			"model", modelComp.String(),
		)

		// make sure the texture dir present. we don't need to do this for the json files,
		// because they live in the same directory as the _template.json files. at this
		// point we know the model dir and item dir exist, because we would have failed
		// earlier when reading the template files.
		if err := dir.MkdirAll(filepath.Join(outDir, w.cfg.TextureDir), os.ModePerm); err != nil {
			return fmt.Errorf("create texture dir: %w", err)
		}

		texture, err := dir.Create(textureFilePath)
		if err != nil {
			return fmt.Errorf("open texture file: %w", err)
		}

		defer texture.Close()

		if err := w.s3store.WriteTo(ctx, blob.CASKeyPrefix+"/"+h, texture); err != nil {
			return fmt.Errorf("write texture file: %w", err)
		}

		if err := dir.WriteFile(itemFilePath, []byte(item), os.ModePerm); err != nil {
			return fmt.Errorf("write thumbnail item: %w", err)
		}

		if err := dir.WriteFile(modelFilePath, []byte(model), os.ModePerm); err != nil {
			return fmt.Errorf("write thumbnail model: %w", err)
		}
	}

	pack, err := dir.OpenFile("latest.zip", os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm)
	if err != nil {
		return fmt.Errorf("open latest.zip file: %w", err)
	}

	defer pack.Close()

	zw := zip.NewWriter(pack)

	// do not use zw.AddFS and walk manually, because AddFS will add filesystem info like
	// date, permissions etc. which will cause the sha1 hash to change even tough he files
	// and their content have not changed. we use the hash to for testing to check for
	// correctness and systems down the line also benefit if the hash only changes if the pack
	// has been actually modified.
	if err := filepath.Walk(mergedDirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(mergedDirPath, path)
		if err != nil {
			return err
		}

		w, err := zw.CreateHeader(&zip.FileHeader{
			Name:   relPath,
			Method: zip.Deflate,
		})
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(w, file)
		return err
	}); err != nil {
		return fmt.Errorf("walk dir: %w", err)
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("close zip writer: %w", err)
	}

	// after writing the zip we need to set the read/write pointers back to the beginning
	// of the file otherwise, we don't read the full data.
	if _, err := pack.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek: %w", err)
	}

	packData, err := io.ReadAll(pack)
	if err != nil {
		return fmt.Errorf("read pack: %w", err)
	}

	// same as above applies, before we can upload we need to move
	// the read pointer to the start, otherwise we upload 0 bytes.
	if _, err := pack.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek: %w", err)
	}

	hash := fmt.Sprintf("%x", sha1.Sum(packData))

	w.logger.InfoContext(ctx, "upload pack", "sha1", hash)

	if err := w.s3store.SimplePut(ctx, uploadKey, pack, map[string]string{
		"sha1": hash,
	}); err != nil {
		return fmt.Errorf("put latest: %w", err)
	}

	return nil
}

func (w *CreateResourcePackWorker) fetchAndUnzipBasePackTo(ctx context.Context, dir *os.Root, mergeDir string) error {
	basePackFile, err := dir.OpenFile(basePackFileName, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm)
	if err != nil {
		return fmt.Errorf("open base pack: %w", err)
	}

	defer basePackFile.Close()

	if err := w.s3store.WriteTo(ctx, w.cfg.PackTemplateKey, basePackFile); err != nil {
		return fmt.Errorf("write base pack: %w", err)
	}

	info, err := basePackFile.Stat()
	if err != nil {
		return fmt.Errorf("stat base pack: %w", err)
	}

	zr, err := zip.NewReader(basePackFile, info.Size())
	if err != nil {
		return fmt.Errorf("zip reader: %w", err)
	}

	dst := filepath.Join(dir.Name(), mergeDir)

	for _, f := range zr.File {
		path := filepath.Join(dst, f.Name)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, os.ModePerm); err != nil {
				return fmt.Errorf("mkdirall: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
			return fmt.Errorf("mkdirall: %w", err)
		}

		out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return fmt.Errorf("open file: %w", err)
		}

		defer out.Close()

		r, err := f.Open()
		if err != nil {
			return fmt.Errorf("open file: %w", err)
		}

		defer r.Close()

		if _, err := io.Copy(out, r); err != nil {
			return fmt.Errorf("copy file: %w", err)
		}
	}

	return nil
}
