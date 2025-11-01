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
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/amacneil/dbmate/v2/pkg/dbmate"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/docker/docker/api/types/container"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spacechunks/explorer/controlplane"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/instance"
	"github.com/spacechunks/explorer/controlplane/postgres"
	"github.com/spacechunks/explorer/internal/image"
	"github.com/spacechunks/explorer/test"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type Postgres struct {
	logger     *slog.Logger
	DB         *postgres.DB
	Pool       *pgxpool.Pool
	ConnString string
}

func NewPostgres() *Postgres {
	return &Postgres{
		logger: slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}
}

func (p *Postgres) Run(t *testing.T, ctx context.Context) {
	var (
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

	p.ConnString = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, ip, mapped.Port(), db)

	u, err := url.Parse(p.ConnString)
	require.NoError(t, err)

	mate := dbmate.New(u)
	mate.MigrationsDir = []string{"../../../controlplane/postgres/migrations"}
	require.NoError(t, mate.Migrate())

	pool, err := pgxpool.New(ctx, p.ConnString)
	require.NoError(t, err)

	// seed data that is globally needed
	_, err = pool.Exec(ctx, `INSERT INTO minecraft_versions (version) VALUES ($1)`, MinecraftVersion)
	require.NoError(t, err)

	p.Pool = pool
	p.DB = postgres.NewDB(p.logger, pool)
}

// CreateRiverClient creates a new river client calling [controlplane.CreateRiverClient]
// and assigning it to the [postgres.DB] instance wrapped by this struct. do not call this
// function if you are using postgres in combination with the control plane fixture as this
// could override the river client set by the control plane.
//
// also needs to be called AFTER [Postgres.Run].
func (p *Postgres) CreateRiverClient(t *testing.T) {
	if p.Pool == nil || p.DB == nil {
		t.Fatal("db connection is nil, call CreateRiverClient after Run")
	}

	var (
		ctx        = context.Background()
		imgService = image.NewService(p.logger, OCIRegsitryUser, OCIRegistryPass, t.TempDir())
		s3client   = NewS3Client(t, ctx)
	)

	riverClient, err := controlplane.CreateRiverClient(
		p.logger,
		p.DB,
		imgService,
		blob.NewS3Store(Bucket, s3client, s3.NewPresignClient(s3client)),
		p.Pool,
		5*time.Second,
		1*time.Second,
		p.DB,
		p.DB,
		runtime.GOOS+"/"+runtime.GOARCH,
	)
	require.NoError(t, err)

	p.DB.SetRiverClient(riverClient)

	err = riverClient.Start(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		riverClient.Stop(ctx)
	})
}

type CreateOptions struct {
	WithFlavors        bool
	WithFlavorVersions bool
}

var CreateOptionsAll = CreateOptions{
	WithFlavors:        true,
	WithFlavorVersions: true,
}

// CreateChunk inserts a chunk and all flavors. it also updates
// the passed object so that dynamically generated values of fields
// like id or created_at have the correct value.
func (p *Postgres) CreateChunk(t *testing.T, c *chunk.Chunk, opts CreateOptions) {
	ctx := context.Background()
	createdChunk, err := p.DB.CreateChunk(ctx, *c)
	require.NoError(t, err)

	if opts.WithFlavors {
		for i := range c.Flavors {
			p.CreateFlavor(t, createdChunk.ID, &createdChunk.Flavors[i], opts)
		}
	}

	*c = createdChunk
}

// CreateFlavor inserts a flavor. it also updates the passed object
// so that dynamically generated values of fields like id or created_at
// have the correct value.
func (p *Postgres) CreateFlavor(t *testing.T, chunkID string, f *chunk.Flavor, opts CreateOptions) {
	var (
		ctx = context.Background()
		tmp = f.Versions // copy here, because CreateFlavor will overwrite it
	)

	createdFlavor, err := p.DB.CreateFlavor(ctx, chunkID, *f)
	require.NoError(t, err)

	createdFlavor.Versions = tmp

	if opts.WithFlavorVersions {
		// insert versions in reverse order to ensure that
		// the latest version actually has the most recent timestamp.
		// since the latest version is the first in the list, inserting
		// without reversing the slice would mean that the latest
		// version has the oldest timestamp and the oldest version has
		// the most recent timestamp.
		slices.Reverse(createdFlavor.Versions)
		for i := range createdFlavor.Versions {
			p.CreateFlavorVersion(t, createdFlavor.ID, &createdFlavor.Versions[i])
		}
		slices.Reverse(createdFlavor.Versions) // reverse again to our desired ordering latest -> oldest
	}

	*f = createdFlavor
}

// CreateInstance inserts an instance and the chunk as well as all flavors
// belonging to the chunk. it also updates the passed object so that dynamically
// generated values of fields like id or created_at have the correct value.
func (p *Postgres) CreateInstance(t *testing.T, nodeID string, ins *instance.Instance) {
	ctx := context.Background()
	p.CreateChunk(t, &ins.Chunk, CreateOptions{
		WithFlavors:        true,
		WithFlavorVersions: true,
	})

	for _, f := range ins.Chunk.Flavors {
		for _, v := range f.Versions {
			// use hash to find flavor version here, because
			// id set in the passed instance will not match
			// due to CreateChunk generating new ids.
			if ins.FlavorVersion.Hash == v.Hash {
				ins.FlavorVersion = v
			}
		}
	}

	created, err := p.DB.CreateInstance(ctx, *ins, nodeID)
	require.NoError(t, err)

	*ins = created
}

func (p *Postgres) CreateFlavorVersion(t *testing.T, flavorID string, version *chunk.FlavorVersion) {
	ctx := context.Background()
	created, err := p.DB.CreateFlavorVersion(ctx, flavorID, *version, "")
	require.NoError(t, err)
	*version = created
}

func (p *Postgres) InsertNode(t *testing.T) {
	ctx := context.Background()
	q := `INSERT INTO nodes (id, name, address, checkpoint_api_endpoint) VALUES ($1, $2, $3, $4)`
	_, err := p.Pool.Exec(ctx, q, Node().ID, Node().Name, Node().Addr, Node().CheckpointAPIEndpoint)
	require.NoError(t, err)
}
