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

package cli

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

func ReadYAMLFile[T any](path string) (T, error) {
	var content T

	data, err := os.ReadFile(path)
	if err != nil {
		return content, fmt.Errorf("couldn't read file: %w", err)
	}

	if err := yaml.Unmarshal(data, &content); err != nil {
		return content, fmt.Errorf("couldn't parse file: %w", err)
	}

	return content, nil
}

func WriteYAMLFile[T any](content T, path string) error {
	data, err := yaml.Marshal(&content)
	if err != nil {
		return fmt.Errorf("couldn't marshal YAML: %w", err)
	}

	if err := os.WriteFile(path, data, 0777); err != nil {
		return fmt.Errorf("couldn't write file: %w", err)
	}

	return nil
}
