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
	"github.com/spacechunks/explorer/cli/cmd/inspect"
	"github.com/spacechunks/explorer/cli/cmd/list"
	"github.com/spacechunks/explorer/cli/cmd/publish"
	"github.com/spacechunks/explorer/cli/cmd/run"
	"github.com/spf13/cobra"
)

func newChunkCommand(ctx context.Context, state cli.State) *cobra.Command {
	c := &cobra.Command{
		Use:   "chunk",
		Short: "TBD",
		Long:  "TBD",
	}

	c.AddCommand(publish.NewCommand(ctx, state))
	c.AddCommand(run.NewCommand(ctx, state))
	c.AddCommand(list.NewCommand(ctx, state))
	c.AddCommand(inspect.NewCommand(ctx, state))

	return c
}
