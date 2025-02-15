/*
 * Explorer Platform, a platform for hosting and discovering Minecraft servers.
 * Copyright (C) 2024 Yannic Rieger <oss@76k.io>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

-- name: CreateChunk :one
INSERT INTO chunks
    (id, name, description, tags)
VALUES
    ($1, $2, $3, $4)
RETURNING *;

-- name: GetChunkByID :one
SELECT * FROM chunks WHERE id = $1;

-- name: UpdateChunk :one
UPDATE chunks
SET
    name = $1,
    description = $2,
    tags = $3,
    updated_at = now()
WHERE id = $4
RETURNING *;
