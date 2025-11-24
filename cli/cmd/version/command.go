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

package version

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) {
		info, ok := debug.ReadBuildInfo()
		if !ok {
			return
		}

		var (
			str    = ""
			dirty  = false
			commit = ""
		)

		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				commit = setting.Value
			case "vcs.modified":
				dirty = true
			}
		}

		str += "Commit: " + commit
		if dirty {
			str += "+dirty"
		}
		fmt.Println(str)
	}

	return &cobra.Command{
		Use:          "version",
		Short:        "Displays version information",
		Run:          run,
		SilenceUsage: true,
	}
}
