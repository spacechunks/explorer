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

	"github.com/spacechunks/explorer/test/functional/fixture"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateChunk(t *testing.T) {
	var (
		ctx = context.Background()
		pg  = fixture.NewPostgres()
	)

	pg.Run(t, ctx)

	expected := fixture.Chunk()

	c, err := pg.DB.CreateChunk(ctx, expected)
	require.NoError(t, err)

	assert.Equal(t, expected.ID, c.ID)
	assert.Equal(t, expected.Name, c.Name)
	assert.Equal(t, expected.Description, c.Description)
	assert.Equal(t, expected.Tags, c.Tags)
	assert.NotEmpty(t, c.CreatedAt)
	assert.NotEmpty(t, c.UpdatedAt)
}
