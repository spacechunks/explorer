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

package tarhelper

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func TarFiles(rootDir string, files []*os.File, dest string) error {
	df, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}

	gzw := gzip.NewWriter(df)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	for _, f := range files {
		info, err := f.Stat()
		if err != nil {
			return fmt.Errorf("stat %s: %w", f.Name(), err)
		}

		// this function will also be called in windows, so we need to adjust the paths
		name := strings.ReplaceAll(
			filepath.ToSlash(f.Name()),
			filepath.Clean(filepath.ToSlash(rootDir))+"/",
			"",
		)

		if err := tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     name,
			Size:     info.Size(),
		}); err != nil {
			return fmt.Errorf("tar header: %w", err)
		}

		if _, err := io.Copy(tw, f); err != nil {
			return fmt.Errorf("copy: %w", err)
		}
	}

	return nil
}

func Untar(r io.Reader, dest string) ([]string, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}

	defer gzr.Close()

	tr := tar.NewReader(gzr)
	paths := make([]string, 0)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar next: %w", err)
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		target := filepath.Join(dest, header.Name)
		paths = append(paths, target)

		if err := func() error {
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			f, err := os.Create(target)
			if err != nil {
				return fmt.Errorf("create file: %w", err)
			}

			defer f.Close()

			if _, err := io.Copy(f, tr); err != nil {
				return fmt.Errorf("copy: %w", err)
			}
			return nil
		}(); err != nil {
			return nil, err
		}
	}

	return paths, nil
}
