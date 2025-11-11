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
	"sync/atomic"

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

type changedFlavor struct {
	onDisk        localFlavor
	prevVersion   string
	addedFiles    []file.Hash
	modifiedFiles []file.Hash
	removedFiles  []file.Hash
}

type localFlavor struct {
	name             string
	version          string
	minecraftVersion string
	path             string
	hash             string
	files            []file.Hash
}

func (f localFlavor) serverRelPath(path string) string {
	return strings.ReplaceAll(path, filepath.Clean(f.path)+"/", "")
}

func (f localFlavor) fileDiff(apiHashes []*chunkv1alpha1.FileHashes) ([]file.Hash, []file.Hash, []file.Hash) {
	prevMap := make(map[string]*chunkv1alpha1.FileHashes, len(apiHashes))
	for _, ah := range apiHashes {
		prevMap[ah.Path] = ah
	}

	local := make(map[string]file.Hash, len(f.files))
	for _, odh := range f.files {
		local[f.serverRelPath(odh.Path)] = odh
	}

	var (
		added    []file.Hash
		modified []file.Hash
		removed  []file.Hash
	)

	for _, prev := range slices.Collect(maps.Values(prevMap)) {
		onDisk, ok := local[prev.Path]
		if ok {
			//  did not change, ignore
			if onDisk.Hash == prev.Hash {
				continue
			}

			modified = append(modified, onDisk)
			continue
		}

		// it does not exist on disk, but was previously present, this means
		// the file has been deleted.
		removed = append(removed, file.Hash{
			Hash: prev.Hash,
			Path: prev.Path,
		})
	}

	for _, onDisk := range slices.Collect(maps.Values(local)) {
		if _, ok := prevMap[f.serverRelPath(onDisk.Path)]; ok {
			continue
		}

		// the on disk file was not previously present, this means it is new
		added = append(added, onDisk)
	}

	return added, modified, removed
}

/*
 * WARNING: The code that follows may make you cry:
 *           A Safety Pig has been provided below for your benefit
 *                              _
 *      _._ _..._ .-',     _.._(`))
 *     '-. `     '  /-._.-'    ',/
 *       )         \            '.
 *      / _    _    |             \
 *     |  a    a    /              |
 *      \   .-.                     ;
 *       '-('' ).-'       ,'       ;
 *          '-;           |      .'
 *            \           \    /
 *            | 7  .__  _.-\   \
 *            | |  |  ``/  /`  /
 *           /,_|  |   /,_/   /
 *              /,_/      '`-'
 */

func NewCommand(ctx context.Context, cliCtx cli.Context) *cobra.Command {
	run := func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(configName)
		if err != nil {
			return fmt.Errorf("couldn't read config file: %w", err)
		}

		var cfg publishConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("couldn't parse config file: %w", err)
		}

		// TODO: get chunk by name
		chunk, err := findChunk(ctx, cliCtx.Client, cfg.Chunk.Name)
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

		resp, err := cliCtx.Client.GetSupportedMinecraftVersions(
			ctx,
			&chunkv1alpha1.GetSupportedMinecraftVersionsRequest{},
		)
		if err != nil {
			return fmt.Errorf("get minecraft versions: %w", err)
		}

		plan := newPlan(cfg, resp.Versions, chunk)
		plan.print()

		if len(plan.addedFlavors)+len(plan.changedFlavors)+len(plan.actionables) == 0 {
			fmt.Println("Nothing to publish.")
			return nil
		}

		if !prompt("Are you sure you want to publish? (y/n):") {
			return nil
		}

		if chunk.Id == "" {
			fmt.Println("Chunk does not exist, creating new Chunk.")
			resp, err := cliCtx.Client.CreateChunk(ctx, &chunkv1alpha1.CreateChunkRequest{
				Name:        cfg.Chunk.Name,
				Description: cfg.Chunk.Description,
				Tags:        cfg.Chunk.Tags,
			})
			if err != nil {
				return fmt.Errorf("error while creating chunk: %w", err)
			}

			chunk = resp.Chunk
		}

		b := builder{
			client:       cliCtx.Client,
			updates:      make(chan buildUpdate),
			buildCounter: &atomic.Int32{},
			changeSetDir: os.TempDir(),
		}

		for _, added := range plan.addedFlavors {
			go b.build(ctx, buildData{
				chunkID: chunk.Id,
				local:   added,
				phase:   buildPhasePrerequisites,
			})
		}

		for _, changed := range plan.changedFlavors {
			go b.build(ctx, buildData{
				chunkID: chunk.Id,
				local:   changed.onDisk,
				phase:   buildPhasePrerequisites,
			})
		}

		for _, a := range plan.actionables {
			go b.build(ctx, buildData{
				chunkID: chunk.Id,
				local:   a.flavor,
				phase:   a.phase,
			})
		}

		updates := make(map[string]buildUpdate)

		fmt.Println("\nNow waiting for updates:")

		// this builds the following line in the terminal and redraws it once we receive an update
		// <flavor1>: <status> | <flavor2>: <status> | <flavor3>: <status> etc...
		b.Wait(ctx, func(u buildUpdate) {
			fmt.Print("\033[2K") // clear current line
			updates[u.data.local.name] = u
			display(updates)
			fmt.Print("\r")
		})

		// have to re-draw again, because we clear the line in Wait
		display(updates)

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

func display(updates map[string]buildUpdate) {
	var (
		keys = slices.Collect(maps.Keys(updates))
		c    = 0
	)

	slices.Sort(keys)

	for _, k := range keys {
		upd := updates[k]
		c++
		fmt.Printf("%s: ", upd.data.local.name)
		if upd.err != nil {
			fmt.Printf("%s%s%s\n", Red, upd.err.Error(), Reset)
		}

		if upd.uploadProgress != nil {
			fmt.Printf("Uploading (%d%%)", *upd.uploadProgress)
		}

		if upd.buildStatus != nil {
			if *upd.buildStatus == "COMPLETED" {
				fmt.Printf("%s%s%s", Green, *upd.buildStatus, Reset)
			} else if strings.Contains(*upd.buildStatus, "FAILED") {
				fmt.Printf("%s%s%s", Red, *upd.buildStatus, Reset)
			} else {
				fmt.Printf("%s", *upd.buildStatus)
			}
		}

		if c != len(updates) {
			fmt.Print(" | ")
		}
	}
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

func localFileHashes(flavorPath string) (string, []file.Hash, error) {
	var (
		fileHashes = make([]file.Hash, 0)
		excluded   = []string{
			filepath.Join("cache", ".*"),
			filepath.Join("versions", ".*"),
			filepath.Join("libraries", ".*"),
			filepath.Join("logs", ".*"),
			filepath.Join("plugins", ".paper-remapped", ".*"),
			"paper.*.jar",
		}
	)

	if err := filepath.WalkDir(flavorPath, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		if err != nil {
			fmt.Printf("Could not walk into directory %s: %v", path, err)
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

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("error while opening file: %w", err)
		}

		defer f.Close()

		hash, err := file.ComputeHashStr(f)
		if err != nil {
			return fmt.Errorf("error while computing hash: %w", err)
		}

		// use file hashes here, so we don't have to keep the whole files content in ram.
		// we'll read the content later again, when uploading the files to the server.
		// drawback here is that if files change in between, the server will reject the
		// uploaded files, but the chances of this happening should be quite small.
		fileHashes = append(fileHashes, file.Hash{
			Path: rel,
			Hash: hash,
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
