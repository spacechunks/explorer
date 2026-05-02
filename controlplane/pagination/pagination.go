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
	"strconv"
)

const DefaultPageSize = 10
const MaxPageSize = 100

func DecodePageToken(pageToken string) (int, error) {
	if pageToken == "" {
		return 0, nil
	}

	offset, err := strconv.Atoi(pageToken)
	if err != nil {
		return 0, err
	}

	if offset < 0 {
		return 0, fmt.Errorf("page token out of range")
	}

	return offset, nil
}

func EncodePageToken(offset int) string {
	return strconv.Itoa(offset)
}

func ResolvePageSize(pageSize uint32) int {
	if pageSize == 0 {
		return DefaultPageSize
	}

	return int(pageSize)
}
