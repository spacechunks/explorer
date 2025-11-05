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

package fixture

import (
	"time"

	"github.com/spacechunks/explorer/controlplane/user"
)

func User(mod ...func(c *user.User)) user.User {
	u := user.User{
		ID:        "019a5637-289e-74ad-b3fb-7534de25e0a9",
		Nickname:  "test-user",
		Email:     "test-user@example.com",
		CreatedAt: time.Date(2025, 11, 5, 13, 12, 15, 0, time.UTC),
		UpdatedAt: time.Date(2025, 11, 11, 10, 26, 0, 0, time.UTC),
	}

	for _, fn := range mod {
		fn(&u)
	}

	return u
}
