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

package publish

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rodaine/table"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	"github.com/spacechunks/explorer/cli"
)

type conflictKind string

const (
	conflictKindVersionExists     conflictKind = "VersionExists"
	conflictKindVersionHashExists conflictKind = "VersionHashExists"
)

type conflict struct {
	kind   conflictKind
	flavor localFlavor
}

type plan struct {
	addedFlavors   []localFlavor
	changedFlavors []changedFlavor
	conflicts      []conflict
}

func newPlan(cfg publishConfig, chunk *chunkv1alpha1.Chunk) (plan, error) {
	p := plan{}

	for _, f := range cfg.Chunk.Flavors {
		hash, fileHashes, err := localFileHashes(f.Path)
		if err != nil {
			return plan{}, fmt.Errorf("error while reading flavors from disk: %w", err)
		}

		local := localFlavor{
			name:    f.Name,
			version: f.Version,
			path:    f.Path,
			files:   fileHashes,
			hash:    hash,
		}

		remote := cli.FindFlavor(chunk.Flavors, func(item *chunkv1alpha1.Flavor) bool {
			return f.Name == item.Name
		})
		if remote == nil {
			p.addedFlavors = append(p.addedFlavors, local)
			continue
		}

		// TODO: exclude from conflicts if flavor version is currently being built
		//       enable retry if build is failed
		if found := slices.ContainsFunc(remote.Versions, func(v *chunkv1alpha1.FlavorVersion) bool {
			return f.Version == v.Version
		}); found {
			p.conflicts = append(p.conflicts, conflict{
				kind:   conflictKindVersionExists,
				flavor: local,
			})
			continue
		}

		if found := slices.ContainsFunc(remote.Versions, func(v *chunkv1alpha1.FlavorVersion) bool {
			return local.hash == v.Hash
		}); found {
			p.conflicts = append(p.conflicts, conflict{
				kind:   conflictKindVersionHashExists,
				flavor: local,
			})
			continue
		}

		prevVersion := remote.Versions[0] // the latest one is always first

		fmt.Println(prevVersion.Version)
		for _, fh := range prevVersion.FileHashes {
			fmt.Println(fh)
		}

		added, changed, removed := local.fileDiff(prevVersion.FileHashes)

		p.changedFlavors = append(p.changedFlavors, changedFlavor{
			onDisk:        local,
			prevVersion:   prevVersion.Version,
			addedFiles:    added,
			modifiedFiles: changed,
			removedFiles:  removed,
		})
	}

	return p, nil
}

func (p plan) print() {
	// New:
	//  + MyFlavor
	//    + Version: v12.04
	//    + Path: /tm/lol
	//    + Files
	//      + /tmp/file1
	//      + /tmp/file2
	// Modified:
	//  ~ MyFlavor2:
	//    ~ Version: v12.04 -> v14.02
	//    ~ Path: /tm/lol
	//    ~ Files:
	//      + /tmp/file1
	//      - /tmp/file2
	//      ~ /tmp/file2
	// Conflicts:
	//  x MyFlavor3
	//	x Flavor version already exists
	//  x MyFlavor 3
	//     x There is already a flavor version having the same files
	//     x Version: v3

	var (
		Reset          = "\033[0m"
		Red            = "\033[31m"
		Green          = "\033[32m"
		Yellow         = "\033[33m"
		addPrefix      = Green + "+" + " "
		modPrefix      = Yellow + "~" + " "
		rmPrefix       = Red + "-" + " "
		conflictPrefix = Red + "x" + " "
		indent1        = " "
		indent2        = "  "
		indent3        = "   "
		tbl            = func() table.Table {
			t := table.New("", "")
			t.WithHeaderFormatter(func(s string, i ...any) string {
				return ""
			})
			return t
		}
	)

	if len(p.addedFlavors) > 0 {
		fmt.Println("New flavors:")
		for _, fl := range p.addedFlavors {
			t := tbl()
			t.AddRow(Green+indent1+fl.name+":", "")
			t.AddRow(indent2+addPrefix+"Version:", fl.version)
			t.AddRow(indent2+addPrefix+"Path:", fl.path)
			t.AddRow(indent2+addPrefix+"Files:", "")
			t.Print()
			for _, fi := range fl.files {
				fmt.Println(indent3, addPrefix, fi.Path)
			}
		}
	}

	if len(p.changedFlavors) > 0 {
		fmt.Println(Reset + "\nModified flavors:")
		for _, fl := range p.changedFlavors {
			t := tbl()
			t.AddRow(Yellow+indent1+fl.onDisk.name+":", "")
			t.AddRow(indent2+modPrefix+"Version:", fmt.Sprintf("%s -> %s", fl.prevVersion, fl.onDisk.version))
			t.AddRow(indent2+modPrefix+"Path:", fl.onDisk.path)
			t.AddRow(indent2+modPrefix+"Files:", "")
			t.Print()
			for _, path := range fl.addedFiles {
				fmt.Println(indent3, addPrefix, path)
			}
			for _, path := range fl.modifiedFiles {
				fmt.Println(indent3, modPrefix, path)
			}
			for _, path := range fl.removedFiles {
				fmt.Println(indent3, rmPrefix, path)
			}
		}
	}

	if len(p.conflicts) > 0 {
		fmt.Println(Reset + "\nConflicts: ")
		for _, c := range p.conflicts {
			fmt.Printf("%s%s%s:\n", indent1, conflictPrefix, c.flavor.name)

			if c.kind == conflictKindVersionExists {
				fmt.Printf("%s%s A Flavor with version %s already exists.\n", indent2, "-", c.flavor.version)
			}

			if c.kind == conflictKindVersionHashExists {
				fmt.Printf("%s%s A version  with the same set of files exist.\n", indent2, "-")
				// TODO: provide version which is the same
			}
		}
		versions := make([]string, 0, len(p.conflicts))
		for _, c := range p.conflicts {
			versions = append(versions, c.flavor.name)
		}
		fmt.Printf(
			"\nWARNING: Flavors %s contain conflicts and will NOT be published when proceeding.\n",
			strings.Join(versions, ", "),
		)
	}

	fmt.Println(Reset)
}

//func test() {
//	p := plan{
//		addedFlavors: []localFlavor{
//			{
//				name:    "Test1",
//				version: "v1",
//				path:    "/tmp/dawdawd",
//				hash:    "0382z494",
//				files: []file.Hash{
//					{
//						Path: "/tmp/dawdawd",
//						Hash: "",
//					},
//					{
//						Path: "/tmp/dawdawd",
//						Hash: "",
//					},
//					{
//						Path: "/tmp/dawdawd",
//						Hash: "",
//					},
//					{
//						Path: "/tmp/dawdawd",
//						Hash: "",
//					},
//				},
//			},
//			{
//				name:    "Test1",
//				version: "v1",
//				path:    "/tmp/dawdawd",
//				hash:    "0382z494",
//				files: []file.Hash{
//					{
//						Path: "/tmp/dawdawd",
//						Hash: "",
//					},
//					{
//						Path: "/tmp/dawdawd",
//						Hash: "",
//					},
//					{
//						Path: "/tmp/dawdawd",
//						Hash: "",
//					},
//					{
//						Path: "/tmp/dawdawd",
//						Hash: "",
//					},
//				},
//			},
//		},
//		changedFlavors: []changedFlavor{
//			{
//				onDisk: localFlavor{
//					name:    "Test1",
//					version: "v1",
//					path:    "/tmp/dawdawd",
//					hash:    "0382z494",
//				},
//				prevVersion:   "v2",
//				addedFiles:    []string{"/tmp/dawdawd", "/tmp/dawdawd", "/tmp/dawdawd"},
//				modifiedFiles: []string{"/tmp/dawdawd", "/tmp/dawdawd", "/tmp/dawdawd"},
//				removedFiles:  []string{"/tmp/dawdawd", "/tmp/dawdawd", "/tmp/dawdawd"},
//			},
//			{
//				onDisk: localFlavor{
//					name:    "Test1",
//					version: "v1",
//					path:    "/tmp/dawdawd",
//					hash:    "0382z494",
//				},
//				prevVersion:   "v2",
//				addedFiles:    []string{"/tmp/dawdawd", "/tmp/dawdawd", "/tmp/dawdawd"},
//				modifiedFiles: []string{"/tmp/dawdawd", "/tmp/dawdawd", "/tmp/dawdawd"},
//				removedFiles:  []string{"/tmp/dawdawd", "/tmp/dawdawd", "/tmp/dawdawd"},
//			},
//		},
//		conflicts: []conflict{
//			{
//				kind: conflictKindVersionExists,
//				flavor: localFlavor{
//					name:    "Test1",
//					version: "v1",
//					path:    "/tmp/dawdawd",
//				},
//			},
//			{
//				kind: conflictKindVersionHashExists,
//				flavor: localFlavor{
//					name:    "Test1",
//					version: "v1",
//					path:    "/tmp/dawdawd",
//				},
//			},
//		},
//	}
//	p.print()
//}
