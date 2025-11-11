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

package fshelper

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func ConfigHome() (string, error) {
	cfgHome := os.Getenv("XDG_CONFIG_HOME")
	if cfgHome != "" {
		return cfgHome, nil
	}

	switch runtime.GOOS {
	case "windows":
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("failed to get config dir: %w", err)
		}
		cfgHome = filepath.Join(dir, "explorer")
	case "linux", "darwin":
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get users home directory: %w", err)
		}
		cfgHome = filepath.Join(homeDir, ".config", "explorer")
	}

	if _, err := os.Stat(cfgHome); err == nil {
		return cfgHome, nil
	}

	if err := os.MkdirAll(cfgHome, 0700); err != nil {
		return "", fmt.Errorf("failed to create config home directory %s: %w", cfgHome, err)
	}

	return cfgHome, nil
}
