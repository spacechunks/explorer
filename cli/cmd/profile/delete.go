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
	"path/filepath"

	"github.com/spacechunks/explorer/cli"
	"github.com/spacechunks/explorer/cli/fshelper"
	"github.com/spf13/cobra"
)

func newDeleteCommand(_ context.Context, cliCtx cli.Context) *cobra.Command {
	run := func(cmd *cobra.Command, args []string) error {
		cfgHome, err := fshelper.ConfigHome()
		if err != nil {
			return fmt.Errorf("could not get config home: %w", err)
		}

		profileName := args[0]
		if profileName == "default" {
			return fmt.Errorf("default profile cannot be deleted")
		}

		entries, err := os.ReadDir(cfgHome)
		if err != nil {
			return fmt.Errorf("config dir: %w", err)
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}

			if profileName == e.Name() {
				if cliCtx.State.ActiveProfile == profileName {
					cliCtx.State.UpdateActiveProfile("default")
				}

				if err := os.RemoveAll(filepath.Join(cfgHome, profileName)); err != nil {
					return fmt.Errorf("delete profile dir: %w", err)
				}
				return nil
			}
		}

		return fmt.Errorf("profile %s does not exist", profileName)
	}

	cmd := &cobra.Command{
		Use:          "delete NAME",
		Args:         cobra.ExactArgs(1),
		Short:        "Deletes a CLI profile",
		RunE:         run,
		SilenceUsage: true,
	}

	return cmd
}
