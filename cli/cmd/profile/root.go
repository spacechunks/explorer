/*
A basic matchmaking service for the Chunk Explorer.
Copyright (C) 2026 Yannic Rieger <oss@76k.io>

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
package profile

import (
	"context"

	"github.com/spacechunks/explorer/cli"
	"github.com/spf13/cobra"
)

func NewProfileCommand(ctx context.Context, cliCtx cli.Context) *cobra.Command {
	root := &cobra.Command{
		Use:   "profile",
		Short: "Manage CLI profiles",
	}

	root.AddCommand(
		newCreateCommand(ctx, cliCtx),
		newListCommand(ctx, cliCtx),
		newSetCommand(ctx, cliCtx),
	)

	return root
}
