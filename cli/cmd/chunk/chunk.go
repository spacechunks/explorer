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

package chunk

import (
	"context"

	"github.com/spacechunks/explorer/cli"
	deletechunk "github.com/spacechunks/explorer/cli/cmd/chunk/delete"
	"github.com/spacechunks/explorer/cli/cmd/chunk/inspect"
	"github.com/spacechunks/explorer/cli/cmd/chunk/list"
	"github.com/spacechunks/explorer/cli/cmd/chunk/publish"
	"github.com/spacechunks/explorer/cli/cmd/chunk/run"
	"github.com/spf13/cobra"
)

func NewChunkCommand(ctx context.Context, cliCtx cli.Context) *cobra.Command {
	c := &cobra.Command{
		Use:   "chunk",
		Short: "Commands related to working with Chunks.",
	}

	publishCmd := cli.RequireAccessToken(ctx, cliCtx, publish.NewCommand)
	publishCmd.Flags().StringP("file", "f", "", "Path to the chunk config file")

	c.AddCommand(
		publishCmd,
		cli.RequireAccessToken(ctx, cliCtx, run.NewCommand),
		cli.RequireAccessToken(ctx, cliCtx, list.NewCommand),
		cli.RequireAccessToken(ctx, cliCtx, inspect.NewCommand),
		cli.RequireAccessToken(ctx, cliCtx, deletechunk.NewCommand),
	)
	return c
}
