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
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/spacechunks/explorer/test"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	RegistryUser = "spc"
	RegistryPass = "test123"
)

func RunRegistry(t *testing.T) string {
	var (
		ctx = context.Background()
	)

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         "functests-registry-" + test.RandHexStr(t),
			Image:        "registry:2",
			ExposedPorts: []string{"5000/tcp"},
			Env: map[string]string{
				"REGISTRY_STORAGE":             "inmemory",
				"REGISTRY_AUTH":                "htpasswd",
				"REGISTRY_AUTH_HTPASSWD_REALM": "RegistryRealm",
				"REGISTRY_AUTH_HTPASSWD_PATH":  "/auth/htpasswd",
			},
			HostConfigModifier: func(cfg *container.HostConfig) {
				cfg.AutoRemove = true
			},
			WaitingFor: wait.ForExposedPort(),
			Files: []testcontainers.ContainerFile{
				{
					Reader: bytes.NewReader(
						[]byte("spc:$2y$05$/KTDDQVDzeG7QSzGSkJUHuN6RSMspCuNMDjaTU96Azb8y8tiuYicW"), // spc:test123
					),
					ContainerFilePath: "/auth/htpasswd",
				},
			},
		},
		Started: true,
	})

	require.NoError(t, err)

	ip, err := ctr.Host(ctx)
	require.NoError(t, err)

	mapped, err := ctr.MappedPort(ctx, "5000")
	require.NoError(t, err)

	return fmt.Sprintf("http://%s:%s", ip, mapped.Port())
}
