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

package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	"github.com/spacechunks/explorer/cli"
	"github.com/spacechunks/explorer/internal/file"
	"github.com/spf13/cobra"
)

const configName = ".chunk.yaml"

type publishConfig struct {
	Version string      `json:"version"`
	Chunk   chunkConfig `json:"chunk"`
}

type chunkConfig struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Tags        []string       `json:"tags"`
	Flavors     []flavorConfig `json:"flavors"`
}

type flavorConfig struct {
	Name             string `json:"name"`
	Version          string `json:"version"`
	MinecraftVersion string `json:"minecraftVersion"`
	Path             string `json:"path"`
}

type localFlavor struct {
	name    string
	version string
	path    string
	hash    string
	files   []file.Hash
}

func (f localFlavor) serverRelPath(path string) string {
	return strings.ReplaceAll(path, filepath.Clean(f.path)+"/", "")
}

func (f localFlavor) fileDiff(apiHashes []*chunkv1alpha1.FileHashes) ([]string, []string, []string) {
	prevMap := make(map[string]*chunkv1alpha1.FileHashes, len(apiHashes))
	for _, ah := range apiHashes {
		prevMap[ah.Path] = ah
	}

	local := make(map[string]file.Hash, len(f.files))
	for _, odh := range f.files {
		local[f.serverRelPath(odh.Path)] = odh
	}

	var (
		added    []string
		modified []string
		removed  []string
	)

	for _, prev := range slices.Collect(maps.Values(prevMap)) {
		onDisk, ok := local[prev.Path]
		if ok {
			//  did not change, ignore
			if onDisk.Hash == prev.Hash {
				continue
			}

			modified = append(modified, onDisk.Path)
			continue
		}

		// it does not exist on disk, but was previously present, this means
		// the file has been deleted.
		removed = append(removed, prev.Path)
	}

	for _, onDisk := range slices.Collect(maps.Values(local)) {
		if _, ok := prevMap[f.serverRelPath(onDisk.Path)]; ok {
			continue
		}

		// the on disk file was not previously present, this means it is new
		added = append(added, onDisk.Path)
	}

	return added, modified, removed
}

type changedFlavor struct {
	onDisk        localFlavor
	prevVersion   string
	addedFiles    []string
	modifiedFiles []string
	removedFiles  []string
}

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
		rmPrefix       = Red + "-" + Reset + " "
		conflictPrefix = Red + "x" + " "
		indent1        = "  "
		indent2        = "    "
		indent3        = "      "
	)

	if len(p.addedFlavors) > 0 {
		fmt.Println("\nNew flavors: ")
		for _, fl := range p.addedFlavors {
			fmt.Printf("%s%s%s%s:\n", indent1, addPrefix, Green, fl.name)
			fmt.Printf("%s%sVersion: %s\n", indent2, addPrefix, fl.version)
			fmt.Printf("%s%sPath: %s\n", indent2, addPrefix, fl.path)
			fmt.Printf("%s%sFiles:\n", indent2, addPrefix)
			for _, fi := range fl.files {
				fmt.Printf("%s%s%s%s\n", indent3, addPrefix, Green, fi.Path)
			}
		}
	}

	if len(p.changedFlavors) > 0 {
		fmt.Println(Reset + "\nModified flavors: ")
		for _, fl := range p.changedFlavors {
			fmt.Printf("%s%s%s:\n", indent1, modPrefix, fl.onDisk.name)
			fmt.Printf("%s%sVersion: %s -> %s\n", indent2, modPrefix, fl.prevVersion, fl.onDisk.version)
			fmt.Printf("%s%sPath: %s\n", indent2, modPrefix, fl.onDisk.path)
			fmt.Printf("%s%sFiles:\n", indent2, modPrefix)
			for _, path := range fl.addedFiles {
				fmt.Printf("%s%s%s%s\n", indent3, addPrefix, Green, path)
			}
			for _, path := range fl.modifiedFiles {
				fmt.Printf("%s%s%s%s\n", indent3, modPrefix, Yellow, path)
			}
			for _, path := range fl.removedFiles {
				fmt.Printf("%s%s%s%s\n", indent3, rmPrefix, Red, path)
			}
		}
	}

	if len(p.conflicts) > 0 {
		fmt.Println(Reset + "\nConflicts: ")
		for _, c := range p.conflicts {
			fmt.Printf("%s%s%s:\n", indent1, conflictPrefix, c.flavor.name)

			if c.kind == conflictKindVersionExists {
				fmt.Printf("%s%s A Flavor with version %s already exists.\n", indent2, conflictPrefix, c.flavor.version)
			}

			if c.kind == conflictKindVersionHashExists {
				fmt.Printf("%s%s A version  with the same set of files exist.\n", indent2, conflictPrefix)
				// TODO: provide version which is the same
			}
		}
		versions := make([]string, 0, len(p.conflicts))
		for _, c := range p.conflicts {
			versions = append(versions, c.flavor.name)
		}
		fmt.Printf(
			Red+"\nWARNING: Flavors %s contain conflicts and will NOT be published when proceeding.\n",
			strings.Join(versions, ", "),
		)
	}

	fmt.Println(Reset)
}

func publish(ctx context.Context, state cli.State) *cobra.Command {
	run := func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(configName)
		if err != nil {
			return fmt.Errorf("couldn't read config file: %w", err)
		}

		var cfg publishConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("couldn't parse config file: %w", err)
		}

		// TODO: validate that only supported minecraft versions can be used
		//       minecraft version concept needs implementation in control plane
		//		 as well. (add field MinecraftVersion field to flavor version).

		// TODO: get chunk by name
		chunk, err := findChunk(ctx, state.Client, cfg.Chunk.Name)
		if err != nil {
			return fmt.Errorf("error while finding chunk: %w", err)
		}

		// TODO: check owner of chunk, if owner is not the current user
		//       fail with error message that chunk already exists.

		if chunk == nil {
			chunk = &chunkv1alpha1.Chunk{
				Name:        cfg.Chunk.Name,
				Description: cfg.Chunk.Description,
				Tags:        cfg.Chunk.Tags,
			}
		}

		plan, err := createPlan(cfg, chunk)
		if err != nil {
			return fmt.Errorf("error while creating plan: %w", err)
		}

		plan.print()

		if !prompt("Are you sure you want to publish? (y/n):") {
			return nil
		}

		if chunk.Id == "" {
			fmt.Println("Chunk does not exist, creating new Chunk.")
			resp, err := state.Client.CreateChunk(ctx, &chunkv1alpha1.CreateChunkRequest{
				Name:        cfg.Chunk.Name,
				Description: cfg.Chunk.Description,
				Tags:        cfg.Chunk.Tags,
			})
			if err != nil {
				return fmt.Errorf("error while creating chunk: %w", err)
			}

			chunk = resp.Chunk
		}

		for _, added := range plan.addedFlavors {
			if err := uploadFiles(ctx, state, chunk.Id, added, true); err != nil {
				return fmt.Errorf("error while uploading files: %w", err)
			}
		}

		for _, changed := range plan.changedFlavors {
			if err := uploadFiles(ctx, state, chunk.Id, changed.onDisk, false); err != nil {
				return fmt.Errorf("error while uploading changed files: %w", err)
			}
		}

		return nil
	}
	return &cobra.Command{
		Use:          "publish",
		Short:        "TBD",
		Long:         "TBD",
		RunE:         run,
		SilenceUsage: true,
	}
}

func createPlan(cfg publishConfig, chunk *chunkv1alpha1.Chunk) (plan, error) {
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

		remote := findFlavor(chunk.Flavors, func(item *chunkv1alpha1.Flavor) bool {
			return f.Name == item.Name
		})
		if remote == nil {
			p.addedFlavors = append(p.addedFlavors, local)
			continue
		}

		fmt.Println(remote.Name)

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

func uploadFiles(ctx context.Context, state cli.State, chunkID string, local localFlavor, createRemote bool) error {
	// TODO: remove create flavor call, because we can create the flavor
	//       when creating the flavor version if needed.

	var flavorID string
	if createRemote {
		resp, err := state.Client.CreateFlavor(ctx, &chunkv1alpha1.CreateFlavorRequest{
			ChunkId: chunkID,
			Name:    local.name,
		})
		if err != nil {
			return fmt.Errorf("error while creating flavor: %w", err)
		}
		flavorID = resp.Flavor.Id
	} else {
		resp, err := state.Client.GetChunk(ctx, &chunkv1alpha1.GetChunkRequest{
			Id: chunkID,
		})
		if err != nil {
			return fmt.Errorf("error while creating flavor: %w", err)
		}
		f := findFlavor(resp.Chunk.Flavors, func(f *chunkv1alpha1.Flavor) bool {
			return f.Name == local.name
		})
		flavorID = f.Id
	}

	hashes := make([]*chunkv1alpha1.FileHashes, 0, len(local.files))
	for _, fh := range local.files {
		hashes = append(hashes, &chunkv1alpha1.FileHashes{
			Path: local.serverRelPath(fh.Path),
			Hash: fh.Hash,
		})
	}

	versionReq, err := state.Client.CreateFlavorVersion(ctx, &chunkv1alpha1.CreateFlavorVersionRequest{
		FlavorId: flavorID,
		Version: &chunkv1alpha1.FlavorVersion{
			Version:    local.version,
			Hash:       local.hash,
			FileHashes: hashes,
		},
	})
	if err != nil {
		return fmt.Errorf("error while creating flavor version: %w", err)
	}

	files := make([]*chunkv1alpha1.File, 0, len(local.files))
	for _, f := range local.files {
		isAdded := slices.ContainsFunc(versionReq.AddedFiles, func(added *chunkv1alpha1.FileHashes) bool {
			return added.Hash == f.Hash
		})

		isChanged := slices.ContainsFunc(versionReq.ChangedFiles, func(changed *chunkv1alpha1.FileHashes) bool {
			return changed.Hash == f.Hash
		})

		if isAdded || isChanged {
			localPath := filepath.Join(local.path, f.Path)
			data, err := os.ReadFile(localPath)
			if err != nil {
				return fmt.Errorf("error while reading file %s: %w", localPath, err)
			}
			files = append(files, &chunkv1alpha1.File{
				Path: local.serverRelPath(f.Path),
				Data: data,
			})
		}
	}

	fmt.Printf("Uploading files for %s...\n", local.name)

	if _, err := state.Client.SaveFlavorFiles(ctx, &chunkv1alpha1.SaveFlavorFilesRequest{
		FlavorVersionId: versionReq.Version.Id,
		Files:           files,
	}); err != nil {
		return fmt.Errorf("error while saving flavor files: %w", err)
	}

	return nil
}

func findChunk(ctx context.Context, c chunkv1alpha1.ChunkServiceClient, name string) (*chunkv1alpha1.Chunk, error) {
	resp, err := c.ListChunks(ctx, &chunkv1alpha1.ListChunksRequest{})
	if err != nil {
		return nil, err
	}
	for _, chunk := range resp.Chunks {
		if chunk.Name != name {
			continue
		}
		return chunk, nil
	}
	return nil, nil
}

func findFlavor(flavors []*chunkv1alpha1.Flavor, filter func(f *chunkv1alpha1.Flavor) bool) *chunkv1alpha1.Flavor {
	for _, f := range flavors {
		if !filter(f) {
			continue
		}
		return f
	}
	return nil
}

func localFileHashes(flavorPath string) (string, []file.Hash, error) {
	var (
		fileHashes = make([]file.Hash, 0)
		excluded   = []string{
			"cache/.*",
			"versions/.*",
			"libraries/.*",
			"logs/.*",
			"paper.*.jar",
			"plugins/.paper-remapped/.*",
		}
	)

	if err := filepath.WalkDir(flavorPath, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		for _, p := range excluded {
			matched, err := regexp.Match(p, []byte(path))
			if err != nil {
				return fmt.Errorf("error while matching pattern %s: %w", p, err)
			}
			// TODO: debug log excluded files
			if matched {
				return nil
			}
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// the containers that are being built by the controlplane
		// are linux only, so use the linux path separator, if we are
		// using the cli on windows or any other platform that does not
		// use "/".
		tmp := strings.ReplaceAll(path, string(os.PathSeparator), "/")

		// exclude the user specific portion of the path so we are left with
		// the path relative to the server root. for example if a plugin in the
		// flavor is located at /home/some_user/my_chunk/flavor1/plugins/myplugin.jar
		// we remove everything so we are left with only plugins/myplugin.jar
		rel := strings.ReplaceAll(tmp, filepath.Clean(flavorPath)+"/", "")

		// use file hashes here, so we don't have to keep the whole files content in ram.
		// we'll read the content later again, when uploading the files to the server.
		// drawback here is that if files change in between, the server will reject the
		// uploaded files, but the chances of this happening should be quite small.
		fileHashes = append(fileHashes, file.Hash{
			Path: rel,
			Hash: file.ComputeHashStr(data),
		})

		return nil
	}); err != nil {
		return "", nil, err
	}

	file.SortHashes(fileHashes)

	tree, err := file.HashTree(fileHashes)
	if err != nil {
		return "", nil, err
	}

	return file.HashTreeRootString(tree), fileHashes, nil
}

func prompt(label string) bool {
	var s string
	r := bufio.NewReader(os.Stdin)
	for {
		_, _ = fmt.Fprint(os.Stdout, label+" ")
		s, _ = r.ReadString('\n')
		s = strings.TrimSpace(s)
		if strings.ToLower(s) == "yes" || strings.ToLower(s) == "y" {
			return true
		}
		if strings.ToLower(s) == "no" || strings.ToLower(s) == "n" {
			return false
		}
	}
}
