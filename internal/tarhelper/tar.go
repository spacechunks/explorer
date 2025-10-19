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
)

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
