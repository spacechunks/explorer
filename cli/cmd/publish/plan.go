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

	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	"github.com/spacechunks/explorer/cli"
)

var (
	Reset          = "\033[0m"
	Red            = "\033[31m"
	Green          = "\033[32m"
	Yellow         = "\033[33m"
	Cyan           = "\033[36m"
	addPrefix      = Green + "+" + " "
	modPrefix      = Yellow + "~" + " "
	rmPrefix       = Red + "-" + " "
	conflictPrefix = Red + "x" + " "
	indent1        = " "
	indent2        = "  "
	indent3        = "   "
)

type conflict interface {
	Print()
	Flavor() localFlavor
}

type versionMismatchConflict struct {
	flavor     localFlavor
	remoteHash string
}

func (c versionMismatchConflict) Flavor() localFlavor {
	return c.flavor
}

func (c versionMismatchConflict) Print() {
	fmt.Printf("%s - Hash of local files differs from what is found in the control plane.\n", indent2)
	fmt.Printf("%s   This is caused by chaning the local files.\n", indent2)
	fmt.Printf("%s   Local: %s, Control plane: %s. \n", indent2, c.flavor.hash, c.remoteHash)
}

type versionExistConflict struct {
	flavor localFlavor
}

func (c versionExistConflict) Flavor() localFlavor {
	return c.flavor
}

func (c versionExistConflict) Print() {
	fmt.Printf("%s - A Flavor with version %s already exists.\n", indent2, c.flavor.version)
}

type versionHashExistsConflict struct {
	flavor localFlavor
}

func (c versionHashExistsConflict) Flavor() localFlavor {
	return c.flavor
}

func (c versionHashExistsConflict) Print() {
	fmt.Printf("%s - A version  with the same set of files exist.\n", indent2)
}

type actionable struct {
	flavor localFlavor
	phase  buildPhase
}

type plan struct {
	addedFlavors   []localFlavor
	changedFlavors []changedFlavor
	actionables    []actionable
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

		remote := cli.Find(chunk.Flavors, func(item *chunkv1alpha1.Flavor) bool {
			return f.Name == item.Name
		})

		if remote == nil {
			p.addedFlavors = append(p.addedFlavors, local)
			continue
		}

		// first we should check if there are any conflicts

		remoteVersion := cli.Find(remote.Versions, func(v *chunkv1alpha1.FlavorVersion) bool {
			return f.Version == v.Version
		})

		// now we need to check if we have to re-add or if the current flavor we have on disk
		// is to be considered "changed"

		if remoteVersion == nil {
			if c := checkHashes(local, remote); c != nil {
				p.conflicts = append(p.conflicts, c)
				continue
			}

			// if the remote version has not been created yet, BUT
			// there are previous versions present, it means that
			// this flavor has changed.
			if len(remote.Versions) > 0 {
				var (
					prevVersion             = remote.Versions[0] // the latest published one is always first
					added, changed, removed = local.fileDiff(prevVersion.FileHashes)
				)

				p.changedFlavors = append(p.changedFlavors, changedFlavor{
					onDisk:        local,
					prevVersion:   prevVersion.Version,
					addedFiles:    added,
					modifiedFiles: changed,
					removedFiles:  removed,
				})
				continue
			}

			// if the remote version is missing, BUT we don't have any previous versions
			// could indicate, that calling the create flavor version endpoint did not
			// succeed.
			p.addedFlavors = append(p.addedFlavors, local)
			continue
		}

		// at this point we reached a state where the version has been successfully
		// created in the control plane, but now we need to figure out some key things:
		// - have the files been uploaded? no -> retry
		// - are we already in a building the version? yes -> just watch
		// - has the build failed? yes -> retry

		if !remoteVersion.FilesUploaded {
			if local.hash != remoteVersion.Hash {
				p.conflicts = append(p.conflicts, versionMismatchConflict{
					flavor:     local,
					remoteHash: remoteVersion.Hash,
				})
				continue
			}

			p.actionables = append(p.actionables, actionable{
				flavor: local,
				phase:  buildPhaseUpload,
			})
			continue
		}

		// if the version exists on the controlplane, files have been uploaded, BUT
		// we find changes in the local filesystem => notify the user that this version
		// already exists on the control plane, because we can assume that the user
		// wanted to publish changes, but forgot to bump the version.
		if found := slices.ContainsFunc(remote.Versions, func(v *chunkv1alpha1.FlavorVersion) bool {
			return local.hash != v.Hash && v.Version == local.version
		}); found {
			p.conflicts = append(p.conflicts, versionExistConflict{
				flavor: local,
			})
			continue
		}

		if remoteVersion.BuildStatus == chunkv1alpha1.BuildStatus_COMPLETED {
			continue
		}

		if remoteVersion.BuildStatus == chunkv1alpha1.BuildStatus_PENDING ||
			remoteVersion.BuildStatus == chunkv1alpha1.BuildStatus_IMAGE_BUILD ||
			remoteVersion.BuildStatus == chunkv1alpha1.BuildStatus_CHECKPOINT_BUILD {
			p.actionables = append(p.actionables, actionable{
				flavor: local,
				phase:  buildPhaseBuildComplete,
			})
			continue
		}

		p.actionables = append(p.actionables, actionable{
			flavor: local,
			phase:  buildPhaseTriggerBuild,
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

	if (len(p.addedFlavors) == 0) && (len(p.changedFlavors) == 0) && (len(p.actionables) == 0) && (len(p.conflicts) == 0) {
		fmt.Println("Nothing to do.")
		return
	}

	if len(p.addedFlavors) > 0 {
		fmt.Println("New flavors:")
		for _, fl := range p.addedFlavors {
			sec := cli.Section()
			sec.AddRow(Green+indent1+fl.name+":", "")
			sec.AddRow(indent2+addPrefix+"Version:", fl.version)
			sec.AddRow(indent2+addPrefix+"Path:", fl.path)
			sec.AddRow(indent2+addPrefix+"Files:", "")
			sec.Print()
			for _, fi := range fl.files {
				fmt.Println(indent3, addPrefix, fi.Path)
			}
		}
	}

	if len(p.changedFlavors) > 0 {
		fmt.Println(Reset + "\nModified flavors:")
		for _, fl := range p.changedFlavors {
			sec := cli.Section()
			sec.AddRow(Yellow+indent1+fl.onDisk.name+":", "")
			sec.AddRow(indent2+modPrefix+"Version:", fmt.Sprintf("%s -> %s", fl.prevVersion, fl.onDisk.version))
			sec.AddRow(indent2+modPrefix+"Path:", fl.onDisk.path)
			sec.AddRow(indent2+modPrefix+"Files:", "")
			sec.Print()
			for _, fh := range fl.addedFiles {
				fmt.Println(indent3, addPrefix, fh.Path)
			}
			for _, fh := range fl.modifiedFiles {
				fmt.Println(indent3, modPrefix, fh.Path)
			}
			for _, fh := range fl.removedFiles {
				fmt.Println(indent3, rmPrefix, fh.Path)
			}
		}
	}

	if len(p.actionables) > 0 {
		fmt.Println(Reset + "\nActions to be performed for the following flavors: ")
		for _, a := range p.actionables {
			sec := cli.Section()
			if a.phase == buildPhaseUpload {
				sec.AddRow(Cyan+indent1+a.flavor.name+" => ", "Retry uploading files")
			}
			if a.phase == buildPhaseTriggerBuild {
				sec.AddRow(Cyan+indent1+a.flavor.name+" => ", "Retry triggering build")
			}
			sec.Print()
		}
	}

	if len(p.conflicts) > 0 {
		fmt.Println(Reset + "\nConflicts: ")
		for _, c := range p.conflicts {
			fmt.Printf("%s%s%s:\n", indent1, conflictPrefix, c.Flavor().name)
			c.Print()
		}
		versions := make([]string, 0, len(p.conflicts))
		for _, c := range p.conflicts {
			versions = append(versions, c.Flavor().name)
		}
		fmt.Printf(
			"\nWARNING: Flavors %s contain conflicts and will NOT be published when proceeding.\n",
			strings.Join(versions, ", "),
		)
	}

	fmt.Println(Reset)
}

func checkHashes(local localFlavor, remote *chunkv1alpha1.Flavor) conflict {
	if found := slices.ContainsFunc(remote.Versions, func(v *chunkv1alpha1.FlavorVersion) bool {
		return local.hash == v.Hash
	}); found {
		return versionHashExistsConflict{
			flavor: local,
		}
	}
	return nil
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
