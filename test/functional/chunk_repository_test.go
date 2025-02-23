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

package functional

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/spacechunks/explorer/test/functional/fixture"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateChunk(t *testing.T) {
	var (
		ctx   = context.Background()
		_, db = fixture.RunDB(t)
	)

	expected := fixture.Chunk

	c, err := db.CreateChunk(ctx, expected)
	require.NoError(t, err)

	assert.Equal(t, expected.ID, c.ID)
	assert.Equal(t, expected.Name, c.Name)
	assert.Equal(t, expected.Description, c.Description)
	assert.Equal(t, expected.Tags, c.Tags)
	assert.NotEmpty(t, c.CreatedAt)
	assert.NotEmpty(t, c.UpdatedAt)
}

func TestCreateInstance(t *testing.T) {
	var (
		ctx      = context.Background()
		pool, db = fixture.RunDB(t)
	)

	nodeID, err := uuid.NewV7()
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `INSERT INTO nodes (id, address) VALUES ($1, $2)`, nodeID.String(), "198.51.100.1")
	require.NoError(t, err)

	_, err = db.CreateChunk(ctx, fixture.Chunk)
	require.NoError(t, err)

	_, err = db.CreateFlavor(ctx, fixture.Flavor1, fixture.Chunk.ID)
	require.NoError(t, err)

	// ^ above are prerequisites

	expected := fixture.Instance

	actual, err := db.CreateInstance(ctx, expected, nodeID.String())
	require.NoError(t, err)

	assert.Equal(t, expected.ID, actual.ID)
	assert.Equal(t, expected.State, actual.State)
	assert.Equal(t, expected.Address, actual.Address)
	assert.Equal(t, expected.ChunkFlavor.Name, actual.ChunkFlavor.Name)
	assert.Equal(t, expected.ChunkFlavor.BaseImageURL, actual.ChunkFlavor.BaseImageURL)
	assert.Equal(t, expected.ChunkFlavor.CheckpointImageURL, actual.ChunkFlavor.CheckpointImageURL)
}
