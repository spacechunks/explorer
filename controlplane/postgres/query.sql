/*
 * CHUNKS
 */

-- name: CreateChunk :one
INSERT INTO chunks
    (id, name, description, tags, created_at, updated_at)
VALUES
    ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- TODO: read multiple
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

-- TODO: insert multiple (aka :batchmany)
-- name: CreateFlavor :one
INSERT INTO flavors
    (id, chunk_id, name, created_at, updated_at)
VALUES
    ($1, $2, $3, $4, $5)
RETURNING *;

-- name: LatestFlavorVersionByFlavorID :one
SELECT * FROM flavor_versions WHERE flavor_id = $1
ORDER BY created_at DESC LIMIT 1;

-- name: FlavorVersionByHash :one
SELECT version FROM flavor_versions WHERE hash = $1;

-- name: FlavorVersionExists :exec
SELECT EXISTS(
    SELECT 1 FROM flavor_versions
    WHERE version = $1 AND flavor_id = $2
);

-- name: FlavorVersionFileHashes :many
SELECT * FROM flavor_version_files WHERE flavor_version_id = $1;

-- name: CreateFlavorVersion :exec
INSERT INTO flavor_versions
    (id, flavor_id, hash, version, prev_version_id)
VALUES
    ($1, $2, $3, $4, $5);

-- name: BulkInsertFlavorFileHashes :batchexec
INSERT INTO flavor_version_files
    (flavor_version_id, file_hash, file_path)
VALUES
    ($1, $2, $3);

/*
 * INSTANCES
 */

-- name: CreateInstance :exec
INSERT INTO instances
    (id, chunk_id, flavor_id, node_id, state, created_at, updated_at)
VALUES
    ($1, $2, $3, $4, $5, $6, $7);

-- name: GetInstance :many
SELECT * FROM instances i
    JOIN flavors f ON i.chunk_id = f.chunk_id
    JOIN chunks c ON f.chunk_id = c.id
    JOIN nodes n ON i.node_id = n.id
WHERE i.id = $1;

-- name: GetInstancesByNodeID :many
SELECT * FROM instances i
    JOIN flavors f ON i.flavor_id = f.id
    JOIN chunks c ON f.chunk_id = c.id
    JOIN nodes n ON i.node_id = n.id
WHERE i.node_id = $1;

-- name: BulkUpdateInstanceStateAndPort :batchexec
UPDATE instances SET
    state = $1,
    port = $2,
    updated_at = now();

-- name: BulkDeleteInstances :batchexec
DELETE FROM instances WHERE id = $1;