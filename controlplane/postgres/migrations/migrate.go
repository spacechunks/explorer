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

package migrations

import (
	"embed"
	"fmt"
	"io"
	"net/url"

	_ "github.com/amacneil/dbmate/v2/pkg/driver/postgres"

	"github.com/amacneil/dbmate/v2/pkg/dbmate"
)

//go:embed *.sql
var fs embed.FS

func Migrate(dsn string) error {
	pgDSN, err := url.Parse(dsn)
	if err != nil {
		return fmt.Errorf("parse dsn: %w", err)
	}

	mate := dbmate.New(pgDSN)
	mate.FS = fs
	mate.Log = io.Discard
	mate.MigrationsDir = []string{"./"}

	if _, err := mate.FindMigrations(); err != nil {
		return fmt.Errorf("find migrations: %w", err)
	}

	if err := mate.Wait(); err != nil {
		return fmt.Errorf("wait migrations: %w", err)
	}

	if err := mate.Migrate(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	return nil
}
