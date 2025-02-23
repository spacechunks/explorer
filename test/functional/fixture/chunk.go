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

package fixture

import (
	"net/netip"
	"time"

	"github.com/spacechunks/explorer/controlplane/chunk"
)

var (
	Flavor1 = chunk.Flavor{
		ID:                 "019532bb-5582-7608-9a08-bb742a8174aa",
		Name:               "flavor1",
		BaseImageURL:       "https://some/url/to/base/img",
		CheckpointImageURL: "https://some/url/to/checkpoint/img",
		CreatedAt:          time.Date(2025, 2, 23, 13, 12, 15, 0, time.UTC),
		UpdatedAt:          time.Date(2025, 2, 28, 10, 26, 0, 0, time.UTC),
	}

	Flavor2 = chunk.Flavor{
		ID:                 "019532bb-73ca-755e-b9c8-96b984ceac42",
		Name:               "flavor2",
		BaseImageURL:       "https://some/url/to/base/img",
		CheckpointImageURL: "https://some/url/to/checkpoint/img",
		CreatedAt:          time.Date(2025, 2, 23, 13, 12, 15, 0, time.UTC),
		UpdatedAt:          time.Date(2025, 2, 28, 10, 26, 0, 0, time.UTC),
	}

	Chunk = chunk.Chunk{
		ID:          "019532bb-2bd3-73ea-afc2-99f368a3eb97",
		Name:        "chunk-fixture",
		Description: "some description bla bla",
		Tags:        []string{"tag1", "tag2"},
		Flavors: []chunk.Flavor{
			Flavor1,
			Flavor2,
		},
		CreatedAt: time.Date(2025, 2, 23, 13, 12, 15, 0, time.UTC),
		UpdatedAt: time.Date(2025, 2, 28, 10, 26, 0, 0, time.UTC),
	}

	Instance = chunk.Instance{
		ID:          "019533f6-a770-7903-8f99-88ae6b271663",
		Chunk:       Chunk,
		ChunkFlavor: Flavor1,
		Address:     netip.MustParseAddr("198.51.100.1"),
		State:       chunk.InstanceStatePending,
		CreatedAt:   time.Date(2025, 2, 23, 13, 12, 15, 0, time.UTC),
		UpdatedAt:   time.Date(2025, 2, 28, 10, 26, 0, 0, time.UTC),
	}
)
