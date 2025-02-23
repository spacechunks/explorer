/*
 * CHUNKS
 */

-- name: CreateChunk :one
INSERT INTO chunks
    (id, name, description, tags)
VALUES
    ($1, $2, $3, $4)
RETURNING *;

-- name: GetChunkByID :many
SELECT * FROM chunks c
    JOIN flavors f ON f.chunk_id = c.id
WHERE c.id = $1;

-- name: UpdateChunk :one
UPDATE chunks
SET
    name = $1,
    description = $2,
    tags = $3,
    updated_at = now()
WHERE id = $4
RETURNING *;

/*
 * FLAVORS
 */

-- name: CreateFlavor :one
INSERT INTO flavors
    (id, chunk_id, name, base_image_url, checkpoint_image_url)
VALUES
    ($1, $2, $3, $4, $5)
RETURNING *;

/*
 * INSTANCES
 */

-- name: CreateInstance :exec
INSERT INTO instances
    (id, flavor_id, node_id)
VALUES
    ($1, $2, $3);

-- name: GetInstance :one
SELECT * FROM instances i
    JOIN flavors f ON i.flavor_id = f.id
    JOIN chunks c ON f.chunk_id = c.id
    JOIN nodes n ON i.node_id = n.id
WHERE i.id = $1;