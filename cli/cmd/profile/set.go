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

package profile

import (
	"context"
	"fmt"
	"os"

	"github.com/spacechunks/explorer/cli"
	"github.com/spacechunks/explorer/cli/fshelper"
	"github.com/spf13/cobra"
)

func newSetCommand(_ context.Context, cliCtx cli.Context) *cobra.Command {
	run := func(cmd *cobra.Command, args []string) error {
		cfgHome, err := fshelper.ConfigHome()
		if err != nil {
			return fmt.Errorf("could not get config home: %w", err)
		}

		profileName := args[0]
		if profileName == "" || profileName == "default" {
			cliCtx.State.UpdateActiveProfile(profileName)
			return nil
		}

		entries, err := os.ReadDir(cfgHome)
		if err != nil {
			return fmt.Errorf("config dir: %w", err)
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}

			if e.Name() != profileName {
				continue
			}

			cliCtx.State.UpdateActiveProfile(profileName)
			return nil
		}

		return fmt.Errorf("profile %s not found", profileName)
	}

	cmd := &cobra.Command{
		Use:          "set NAME",
		Args:         cobra.ExactArgs(1),
		Short:        "sets an active profile",
		RunE:         run,
		SilenceUsage: true,
	}

	return cmd
}
