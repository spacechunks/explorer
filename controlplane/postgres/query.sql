/*
 * NODES
 */

-- name: RandomNode :one
SELECT * FROM nodes ORDER BY random() LIMIT 1;

/*
 * CHUNKS
 */

-- name: CreateChunk :exec
INSERT INTO chunks
    (id, name, description, tags, created_at, updated_at)
VALUES
    ($1, $2, $3, $4, $5, $6);

-- TODO: read multiple
-- name: GetChunkByID :many
SELECT * FROM chunks c
    LEFT JOIN flavors f ON f.chunk_id = c.id
    LEFT JOIN flavor_versions v ON v.flavor_id = f.id
    LEFT JOIN flavor_version_files vf ON vf.flavor_version_id = v.id
WHERE c.id = $1;

-- name: UpdateChunk :exec
UPDATE chunks
SET
    name = $1,
    description = $2,
    tags = $3,
    updated_at = now()
WHERE id = $4;

-- name: ChunkExists :one
SELECT EXISTS(
    SELECT 1 FROM chunks
    WHERE id = $1
);

-- name: ListChunks :many
SELECT * FROM chunks c
    LEFT JOIN flavors f ON f.chunk_id = c.id
    LEFT JOIN flavor_versions v ON v.flavor_id = f.id
    LEFT JOIN flavor_version_files vf ON vf.flavor_version_id = v.id;

/*
 * FLAVORS
 */

-- TODO: insert multiple (aka :batchmany)
-- name: CreateFlavor :exec
INSERT INTO flavors
    (id, chunk_id, name, created_at, updated_at)
VALUES
    ($1, $2, $3, $4, $5);

-- name: ListFlavorsByChunkID :many
SELECT * FROM flavors f
    JOIN flavor_versions v ON v.flavor_id = f.id
    JOIN flavor_version_files vf ON vf.flavor_version_id = v.id
WHERE chunk_id = $1;

-- name: FlavorNameExists :one
SELECT EXISTS(
    SELECT 1 FROM flavors
    WHERE name = $1 AND chunk_id = $2
);

-- name: FlavorVersionByID :many
SELECT * FROM flavor_versions v
    JOIN flavor_version_files f ON f.flavor_version_id = v.id
WHERE id = $1;

-- name: LatestFlavorVersionByFlavorID :one
SELECT * FROM flavor_versions WHERE flavor_id = $1
ORDER BY created_at DESC LIMIT 1;

-- name: FlavorVersionExists :one
SELECT EXISTS(
    SELECT 1 FROM flavor_versions
    WHERE version = $1 AND flavor_id = $2
);

-- name: FlavorVersionFileHashes :many
SELECT * FROM flavor_version_files WHERE flavor_version_id = $1;

-- name: CreateFlavorVersion :exec
INSERT INTO flavor_versions
    (id, flavor_id, hash, change_hash, version, prev_version_id, created_at)
VALUES
    ($1, $2, $3, $4, $5, $6, $7);

-- name: BulkInsertFlavorFileHashes :batchexec
INSERT INTO flavor_version_files
    (flavor_version_id, file_hash, file_path)
VALUES
    ($1, $2, $3);

-- name: FlavorVersionHashByID :one
SELECT hash FROM flavor_versions WHERE id = $1;

-- name: MarkFlavorVersionFilesUploaded :exec
UPDATE flavor_versions SET files_uploaded = TRUE WHERE id = $1;

-- name: UpdateFlavorVersionBuildStatus :exec
UPDATE flavor_versions SET build_status = $1 WHERE id = $2;

-- name: UpdateFlavorVersionPresignedURLData :exec
UPDATE flavor_versions SET
    presigned_url_expiry_date = $1,
    presigned_url = $2
WHERE id = $3;

/*
 * BLOB STORE
 */

-- name: BulkInsertBlobData :batchexec
INSERT INTO blobs
    (hash, data)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: BulkGetBlobData :batchmany
SELECT * FROM blobs WHERE hash = $1;

/*
 * INSTANCES
 */

-- name: CreateInstance :exec
INSERT INTO instances
    (id, chunk_id, flavor_version_id, node_id, state, created_at, updated_at)
VALUES
    ($1, $2, $3, $4, $5, $6, $7);

-- name: ListInstances :many
SELECT * FROM instances i
    JOIN flavor_versions v ON i.flavor_version_id = v.id
    JOIN chunks c ON i.chunk_id = c.id
    JOIN flavors f ON f.chunk_id = c.id
    JOIN nodes n ON i.node_id = n.id;

-- name: GetInstance :many
SELECT * FROM instances i
    JOIN flavor_versions v ON i.flavor_version_id = v.id
    JOIN chunks c ON i.chunk_id = c.id
    JOIN flavors f ON f.chunk_id = c.id
    JOIN nodes n ON i.node_id = n.id
WHERE i.id = $1;

-- name: GetInstancesByNodeID :many
SELECT * FROM instances i
    JOIN flavor_versions v ON i.flavor_version_id = v.id
    JOIN chunks c ON i.chunk_id = c.id
    JOIN nodes n ON i.node_id = n.id
WHERE i.node_id = $1;

-- name: BulkUpdateInstanceStateAndPort :batchexec
UPDATE instances SET
    state = $1,
    port = $2,
    updated_at = now()
WHERE id = $3;

-- name: BulkDeleteInstances :batchexec
DELETE FROM instances WHERE id = $1;

/*
 * MINECRAFT VERSIONS
 */

-- name: AllMinecraftVersions :many
SELECT version FROM minecraft_versions;

