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

package pagination

import (
	"fmt"

	"github.com/google/uuid"
)

const DefaultPageSize = 10
const MaxPageSize = 100

func DecodePageToken(pageToken string) (*string, error) {
	if pageToken == "" {
		return nil, nil
	}

	tokenUUID, err := uuid.Parse(pageToken)
	if err != nil {
		return nil, err
	}

	if tokenUUID.Version() != 7 {
		return nil, fmt.Errorf("page token must be a uuidv7")
	}

	return &pageToken, nil
}

func EncodePageToken(lastID string) string {
	return lastID
}

func ResolvePageSize(pageSize uint32) int {
	if pageSize == 0 {
		return DefaultPageSize
	}

	return int(pageSize)
}
