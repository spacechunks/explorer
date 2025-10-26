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

package cli

import (
	"github.com/rodaine/table"
)

func Find[T any](s []*T, filter func(i *T) bool) *T {
	for _, f := range s {
		if !filter(f) {
			continue
		}
		return f
	}

	return nil
}

// Section is a table with no column headers. the main purpose
// of this is to align values when printing, so they are on the
// same level. here's an example:
// what we don't want:
//
//	ID: 0198fb5f-e59e-7794-87f9-e34bd32a0e1b
//	Name: TestChunk
//	Description: this is a description
//	Tags: tag1,tag2
//
// what we want:
//
//	ID:            0198fb5f-e59e-7794-87f9-e34bd32a0e1b
//	Name:          TestChunk
//	Description:   this is a description
//	Tags:          tag1,tag2
func Section() table.Table {
	t := table.New("", "")
	t.WithHeaderFormatter(func(s string, i ...any) string {
		return ""
	})
	return t
}
