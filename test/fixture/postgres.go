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
	"github.com/spacechunks/explorer/controlplane"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/image"
	"github.com/spacechunks/explorer/controlplane/instance"
	"github.com/spacechunks/explorer/controlplane/postgres"
	"github.com/spacechunks/explorer/test"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type Postgres struct {
	DB         *postgres.DB
	Pool       *pgxpool.Pool
	ConnString string
}

func NewPostgres() *Postgres {
	return &Postgres{}
}

func (p *Postgres) Run(t *testing.T, ctx context.Context) {
	var (
		user   = os.Getenv("FUNCTESTS_POSTGRES_USER")
		pass   = os.Getenv("FUNCTESTS_POSTGRES_PASS")
		db     = os.Getenv("FUNCTESTS_POSTGRES_DB")
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
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
	mate.MigrationsDir = []string{"../../controlplane/postgres/migrations"}
	require.NoError(t, mate.Migrate())

	pool, err := pgxpool.New(ctx, p.ConnString)
	require.NoError(t, err)

	p.Pool = pool
	p.DB = postgres.NewDB(logger, pool)

	var (
		blobStore  = blob.NewPGStore(p.DB)
		imgService = image.NewService(logger, OCIRegsitryUser, OCIRegistryPass, t.TempDir())
	)

	riverClient, err := controlplane.CreateRiverClient(logger, p.DB, imgService, blobStore, pool)
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
		for i := range createdFlavor.Versions {
			p.CreateFlavorVersion(t, createdFlavor.ID, &createdFlavor.Versions[i])
		}
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
		// flavor names for a chunk are unique
		if ins.ChunkFlavor.Name == f.Name {
			ins.ChunkFlavor = f
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

func (p *Postgres) CreateBlobs(t *testing.T, version chunk.FlavorVersion) {
	var objs []blob.Object
	for _, fh := range version.FileHashes {
		objs = append(objs, blob.Object{
			Hash: fh.Hash,
			Data: []byte(fh.Path),
		})
	}

	err := p.DB.BulkWriteBlobs(context.Background(), objs)
	require.NoError(t, err)
}

func (p *Postgres) InsertNode(t *testing.T) {
	ctx := context.Background()
	_, err := p.Pool.Exec(ctx, `INSERT INTO nodes (id, address) VALUES ($1, $2)`, Node().ID, Node().Addr)
	require.NoError(t, err)
}
