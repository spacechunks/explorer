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
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"testing"

	"github.com/amacneil/dbmate/v2/pkg/dbmate"
	"github.com/docker/docker/api/types/container"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spacechunks/explorer/controlplane/postgres"
	"github.com/spacechunks/explorer/test"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	_ "github.com/amacneil/dbmate/v2/pkg/driver/postgres"
)

// grpc client does not accept @ as abstract socket identifier,
// so do not include it in the address string.

const platformdAddr = "/run/platformd/platformd.sock"

func PlatformdClientConn(t *testing.T) *grpc.ClientConn {
	conn, err := grpc.NewClient(
		"unix-abstract:"+platformdAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return conn
}

func RunDB(t *testing.T) (*pgxpool.Pool, *postgres.DB) {
	var (
		ctx  = context.Background()
		user = os.Getenv("FUNCTESTS_POSTGRES_USER")
		pass = os.Getenv("FUNCTESTS_POSTGRES_PASS")
		db   = os.Getenv("FUNCTESTS_POSTGRES_DB")
	)

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         "functests-db-" + test.RandHexStr(t),
			Image:        os.Getenv("FUNCTESTS_POSTGRES_IMAGE"),
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     user,
				"POSTGRES_PASSWORD": pass,
				"POSTGRES_DB":       db,
			},
			HostConfigModifier: func(cfg *container.HostConfig) {
				cfg.AutoRemove = true
			},
			WaitingFor: wait.ForExposedPort(),
		},
		Started: true,
	})
	require.NoError(t, err)

	ip, err := ctr.Host(ctx)
	require.NoError(t, err)

	mapped, err := ctr.MappedPort(ctx, "5432")
	require.NoError(t, err)

	addr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, ip, mapped.Port(), db)

	u, err := url.Parse(addr)
	require.NoError(t, err)

	mate := dbmate.New(u)
	mate.MigrationsDir = []string{"../../controlplane/postgres/migrations"}
	require.NoError(t, mate.Migrate())

	pool, err := pgxpool.New(ctx, addr)
	require.NoError(t, err)

	return pool, postgres.NewDB(slog.New(slog.NewTextHandler(os.Stdout, nil)), pool)
}
