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
	"fmt"
	"hash"
	"log"
	"net/netip"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cbergoon/merkletree"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/instance"
	"github.com/spacechunks/explorer/controlplane/node"
	"github.com/spacechunks/explorer/internal/file"
	"github.com/spacechunks/explorer/internal/ptr"
	"github.com/zeebo/xxh3"
)

const FlavorID = "019532bb-5582-7608-9a08-bb742a8174aa"

func Chunk(mod ...func(c *chunk.Chunk)) chunk.Chunk {
	c := chunk.Chunk{
		ID:          "019532bb-2bd3-73ea-afc2-99f368a3eb97",
		Name:        "chunk-fixture",
		Description: "some description bla bla",
		Tags:        []string{"tag1", "tag2"},
		Flavors: []chunk.Flavor{
			// the latest flavor has to be first
			Flavor(func(f *chunk.Flavor) {
				f.ID = "01953e68-8313-76ba-8012-2f51abb7988a"
				f.Name = "flavor2"
				f.CreatedAt = time.Date(2025, 2, 23, 13, 12, 15, 0, time.UTC)
			}),
			Flavor(func(f *chunk.Flavor) {
				f.ID = "01953e68-4ca6-73b1-89b4-86455ffd78e7"
				f.Name = "flavor1"
				f.CreatedAt = time.Date(2024, 2, 23, 13, 12, 15, 0, time.UTC)
			}),
		},
		CreatedAt: time.Date(2025, 2, 23, 13, 12, 15, 0, time.UTC),
		UpdatedAt: time.Date(2025, 2, 28, 10, 26, 0, 0, time.UTC),
	}

	for _, fn := range mod {
		fn(&c)
	}

	return c
}

func Flavor(mod ...func(f *chunk.Flavor)) chunk.Flavor {
	flavor := chunk.Flavor{
		ID:   "019532bb-5582-7608-9a08-bb742a8174aa",
		Name: "flavorABC",
		Versions: []chunk.FlavorVersion{
			FlavorVersion(nil, func(v *chunk.FlavorVersion) {
				v.ID = "01953e68-4ca6-73b1-89b4-86455ffd78e7"
				v.Version = "v2"
				v.FileHashes = []file.Hash{
					{
						Path: "/tmp/somefile",
						Hash: "aaaaaaaaaaaaaaaa",
					},
				}
			}),
			FlavorVersion(nil, func(v *chunk.FlavorVersion) {
				v.ID = "01953e68-4ca6-73b1-89b4-86455ffd78e7"
				v.Version = "v1"
			}),
		},
		CreatedAt: time.Date(2025, 2, 23, 13, 12, 15, 0, time.UTC),
		UpdatedAt: time.Date(2025, 2, 28, 10, 26, 0, 0, time.UTC),
	}

	for _, fn := range mod {
		fn(&flavor)
	}

	return flavor
}

func FlavorVersion(t *testing.T, mod ...func(v *chunk.FlavorVersion)) chunk.FlavorVersion {
	version := chunk.FlavorVersion{
		Version:    "v1",
		ChangeHash: "kkkkkkkkkkkkkkkk",
		FileHashes: []file.Hash{
			{
				Path: "server.properties",
				Hash: "server-prop-hash", // hashes can only be 16 chars long
			},
			{
				Path: "plugins/myplugin/config.json",
				Hash: "cooooooooooooooo",
			},
			{
				Path: "paper.yml",
				Hash: "pppppppppppppppp",
			},
		},
		BuildStatus:   chunk.BuildStatusPending,
		FilesUploaded: false,
		CreatedAt:     time.Time{},
	}

	for _, mod := range mod {
		mod(&version)
	}

	sort.Slice(version.FileHashes, func(i, j int) bool {
		return strings.Compare(version.FileHashes[i].Path, version.FileHashes[j].Path) < 0
	})

	sorted := make([]file.Hash, len(version.FileHashes))
	copy(sorted, version.FileHashes)

	content := make([]merkletree.Content, 0, len(version.FileHashes))
	for _, f := range version.FileHashes {
		content = append(content, f)
	}

	tree, err := merkletree.NewTreeWithHashStrategy(content, func() hash.Hash {
		return xxh3.New()
	})
	if err != nil {
		log.Fatalf("create merkle tree: %v", err)
	}

	version.Hash = fmt.Sprintf("%x", tree.MerkleRoot())

	// call twice in case we need to modify the hash
	for _, mod := range mod {
		mod(&version)
	}

	// the unsorted slice has been applied again by the loop above
	version.FileHashes = sorted

	return version
}

func Instance(mod ...func(i *instance.Instance)) instance.Instance {
	c := Chunk()
	ins := instance.Instance{
		ID:          "019533f6-a770-7903-8f99-88ae6b271663",
		Chunk:       c,
		ChunkFlavor: c.Flavors[0],
		Address:     netip.MustParseAddr("198.51.100.1"),
		State:       instance.StatePending,
		Port:        ptr.Pointer(uint16(1337)),
		CreatedAt:   time.Date(2025, 2, 23, 13, 12, 15, 0, time.UTC),
		UpdatedAt:   time.Date(2025, 2, 28, 10, 26, 0, 0, time.UTC),
	}

	for _, fn := range mod {
		fn(&ins)
	}

	return ins
}

func Node() node.Node {
	return node.Node{
		ID:                    "0195c2f6-f40c-72df-a0f1-e468f1be77b1",
		Name:                  "test-node",
		Addr:                  netip.MustParseAddr("198.51.100.1"),
		CheckpointAPIEndpoint: netip.MustParseAddrPort(CheckpointAPIAddr),
	}
}
