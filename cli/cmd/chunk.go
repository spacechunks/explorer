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
	"fmt"

	"github.com/spacechunks/explorer/cli"
	"github.com/spacechunks/explorer/cli/cmd/inspect"
	"github.com/spacechunks/explorer/cli/cmd/list"
	"github.com/spacechunks/explorer/cli/cmd/publish"
	"github.com/spacechunks/explorer/cli/cmd/run"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/metadata"
)

func newChunkCommand(ctx context.Context, cliCtx cli.Context) *cobra.Command {
	c := &cobra.Command{
		Use:   "chunk",
		Short: "Commands related to working with Chunks.",
	}
	c.AddCommand(
		requireAPIToken(ctx, cliCtx, publish.NewCommand),
		requireAPIToken(ctx, cliCtx, run.NewCommand),
		requireAPIToken(ctx, cliCtx, list.NewCommand),
		requireAPIToken(ctx, cliCtx, inspect.NewCommand),
	)
	return c
}

func requireAPIToken(
	ctx context.Context,
	cliCtx cli.Context,
	fn func(context.Context, cli.Context) *cobra.Command,
) *cobra.Command {
	cmd := fn(ctx, cliCtx)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		tok, err := cliCtx.Auth.APIToken(ctx)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		md := metadata.Pairs("authorization", tok)
		ctx = metadata.NewOutgoingContext(ctx, md)

		return fn(ctx, cliCtx).RunE(cmd, args)
	}
	return cmd
}
