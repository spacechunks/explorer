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

package inspect

import (
	"context"
	"fmt"
	"strings"
	"time"

	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	"github.com/spacechunks/explorer/cli"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func NewCommand(ctx context.Context, state cli.State) *cobra.Command {
	run := func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("chunk id is missing")
		}

		// TODO: replace by GetChunkRequest with name filter

		resp, err := state.Client.GetChunk(ctx, &chunkv1alpha1.GetChunkRequest{
			Id: args[0],
		})
		if err != nil {
			return fmt.Errorf("error while getting chunk: %w", err)
		}

		found := resp.GetChunk()

		var (
			indent1 = " "
			indent2 = "  "
			indent3 = "   "
			indent4 = "    "
		)

		chunkData := cli.Section()
		chunkData.AddRow("ID: ", found.Id)
		chunkData.AddRow("Name: ", found.Name)
		chunkData.AddRow("Description: ", found.Description)
		chunkData.AddRow("Tags: ", strings.Join(found.Tags, ","))
		chunkData.AddRow("Created at: ", fmtTime(found.CreatedAt))
		chunkData.AddRow("Updated at: ", fmtTime(found.UpdatedAt))
		chunkData.Print()

		fmt.Println("Flavors:")

		for _, f := range found.Flavors {
			flavorData := cli.Section()
			flavorData.AddRow(indent1+f.Name+":", "")
			flavorData.AddRow(indent2+"ID:", f.Id)
			flavorData.AddRow(indent2+"Created at:", fmtTime(f.CreatedAt))
			flavorData.AddRow(indent2+"Updated at:", fmtTime(f.UpdatedAt))
			flavorData.Print()
			fmt.Println(indent2 + "Versions:")
			for _, v := range f.Versions {
				versionData := cli.Section()
				versionData.AddRow(indent3+v.Version+":", "")
				versionData.AddRow(indent4+"ID:", v.Id)
				versionData.AddRow(indent4+"Minecraft version"+":", v.MinecraftVersion)
				versionData.AddRow(indent4+"Created at:", fmtTime(v.CreatedAt))
				versionData.AddRow(indent4+"Build status:", v.BuildStatus)
				versionData.Print()
			}
		}

		return nil
	}

	return &cobra.Command{
		Use:   "inspect",
		Short: "Shows detailed information about a single Chunk",
		RunE:  run,
	}
}

func fmtTime(t *timestamppb.Timestamp) string {
	if t == nil {
		return "Not available"
	}
	return t.AsTime().Format(time.RFC1123Z)
}
