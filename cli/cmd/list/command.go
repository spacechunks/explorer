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

package list

import (
	"context"
	"fmt"
	"strings"

	"github.com/rodaine/table"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	"github.com/spacechunks/explorer/cli"
	"github.com/spf13/cobra"
)

func NewCommand(ctx context.Context, state cli.State) *cobra.Command {
	run := func(cmd *cobra.Command, args []string) error {
		resp, err := state.Client.ListChunks(ctx, &chunkv1alpha1.ListChunksRequest{})
		if err != nil {
			return fmt.Errorf("error while listing chunks: %w", err)
		}

		t := table.New("Name", "Description", "Tags", "ID")
		for _, c := range resp.Chunks {
			t.AddRow(c.Name, c.Description, strings.Join(c.Tags, ","), c.Id)
		}
		t.Print()

		return nil
	}

	return &cobra.Command{
		Use:   "list",
		Short: "List all available chunks",
		RunE:  run,
	}
}
