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

package blob

import (
	"fmt"
	"io"
	"os"

	"github.com/spacechunks/explorer/internal/file"
)

type Object struct {
	Data io.ReadSeekCloser
	hash string
}

func NewFromFile(path string) (Object, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return Object{}, fmt.Errorf("open file: %w", err)
	}

	return Object{
		Data: f,
	}, nil
}

func (o *Object) Hash() (string, error) {
	if o.hash != "" {
		return o.hash, nil
	}

	h, err := file.ComputeHashStr(o.Data)
	if err != nil {
		return "", fmt.Errorf("compute hash: %w", err)
	}

	o.hash = h

	return o.hash, nil
}
