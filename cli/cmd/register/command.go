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

package register

import (
	"context"
	"fmt"

	userv1alpha1 "github.com/spacechunks/explorer/api/user/v1alpha1"
	"github.com/spacechunks/explorer/cli"
	"github.com/spf13/cobra"
)

func NewCommand(ctx context.Context, cliCtx cli.Context) *cobra.Command {
	run := func(cmd *cobra.Command, args []string) error {
		tok, err := cliCtx.Auth.IDToken(ctx)
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		if _, err := cliCtx.UserClient.Register(ctx, &userv1alpha1.RegisterRequest{
			Nickname: args[0],
			IdToken:  tok,
		}); err != nil {
			return fmt.Errorf("register failed: %w", err)
		}

		return nil
	}

	return &cobra.Command{
		Use:          "register NICKNAME",
		Args:         cobra.ExactArgs(1),
		Short:        "Register a new account with the Chunk Explorer.",
		RunE:         run,
		SilenceUsage: true,
	}
}
