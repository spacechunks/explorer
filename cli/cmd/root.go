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
	"context"

	"github.com/spacechunks/explorer/cli"
	"github.com/spacechunks/explorer/cli/cmd/register"
	"github.com/spacechunks/explorer/cli/cmd/version"
	"github.com/spf13/cobra"
)

func Root(ctx context.Context, cliCtx cli.Context) *cobra.Command {
	root := &cobra.Command{
		Use: "explorer",
		Long: `A library of creations, where everyone can share their projects with the world.
A place of discovery and play. All within a single unified system.`,
	}

	chunkCmd := newChunkCommand(ctx, cliCtx)
	root.AddCommand(
		chunkCmd,
		register.NewCommand(ctx, cliCtx),
		version.NewCommand(),
	)

	return root
}
