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
	"fmt"
	"os"

	"github.com/spacechunks/explorer/cli"
	"github.com/spacechunks/explorer/cli/fshelper"
	"github.com/spf13/cobra"
)

func newListCommand(_ context.Context, cliCtx cli.Context) *cobra.Command {
	run := func(cmd *cobra.Command, args []string) error {
		cfgHome, err := fshelper.ConfigHome()
		if err != nil {
			return fmt.Errorf("could not get config home: %w", err)
		}

		entries, err := os.ReadDir(cfgHome)
		if err != nil {
			return fmt.Errorf("config dir: %w", err)
		}

		sec := cli.Section()
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if cliCtx.State.ActiveProfile == e.Name() {

				sec.AddRow(e.Name(), "(active)")
				continue
			}

			sec.AddRow(e.Name())
		}

		sec.Print()
		return nil
	}

	cmd := &cobra.Command{
		Use:          "list",
		Short:        "Lists all CLI profile",
		RunE:         run,
		SilenceUsage: true,
	}

	return cmd
}
