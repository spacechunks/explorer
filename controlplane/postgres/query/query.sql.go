// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0
// source: query.sql

package query

import (
	"context"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

const chunkExists = `-- name: ChunkExists :one
SELECT EXISTS(
    SELECT 1 FROM chunks
    WHERE id = $1
)
`

func (q *Queries) ChunkExists(ctx context.Context, id string) (bool, error) {
	row := q.db.QueryRow(ctx, chunkExists, id)
	var exists bool
	err := row.Scan(&exists)
	return exists, err
}

const createChunk = `-- name: CreateChunk :exec
/*
 * CHUNKS
 */

INSERT INTO chunks
    (id, name, description, tags, created_at, updated_at)
VALUES
    ($1, $2, $3, $4, $5, $6)
`

type CreateChunkParams struct {
	ID          string
	Name        string
	Description string
	Tags        []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (q *Queries) CreateChunk(ctx context.Context, arg CreateChunkParams) error {
	_, err := q.db.Exec(ctx, createChunk,
		arg.ID,
		arg.Name,
		arg.Description,
		arg.Tags,
		arg.CreatedAt,
		arg.UpdatedAt,
	)
	return err
}

const createFlavor = `-- name: CreateFlavor :exec
/*
 * FLAVORS
 */

INSERT INTO flavors
    (id, chunk_id, name, created_at, updated_at)
VALUES
    ($1, $2, $3, $4, $5)
`

type CreateFlavorParams struct {
	ID        string
	ChunkID   string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TODO: insert multiple (aka :batchmany)
func (q *Queries) CreateFlavor(ctx context.Context, arg CreateFlavorParams) error {
	_, err := q.db.Exec(ctx, createFlavor,
		arg.ID,
		arg.ChunkID,
		arg.Name,
		arg.CreatedAt,
		arg.UpdatedAt,
	)
	return err
}

const createFlavorVersion = `-- name: CreateFlavorVersion :exec
INSERT INTO flavor_versions
    (id, flavor_id, hash, change_hash, version, prev_version_id, created_at)
VALUES
    ($1, $2, $3, $4, $5, $6, $7)
`

type CreateFlavorVersionParams struct {
	ID            string
	FlavorID      string
	Hash          string
	ChangeHash    string
	Version       string
	PrevVersionID *string
	CreatedAt     time.Time
}

func (q *Queries) CreateFlavorVersion(ctx context.Context, arg CreateFlavorVersionParams) error {
	_, err := q.db.Exec(ctx, createFlavorVersion,
		arg.ID,
		arg.FlavorID,
		arg.Hash,
		arg.ChangeHash,
		arg.Version,
		arg.PrevVersionID,
		arg.CreatedAt,
	)
	return err
}

const createInstance = `-- name: CreateInstance :exec
/*
 * INSTANCES
 */

INSERT INTO instances
    (id, chunk_id, flavor_id, node_id, state, created_at, updated_at)
VALUES
    ($1, $2, $3, $4, $5, $6, $7)
`

type CreateInstanceParams struct {
	ID        string
	ChunkID   string
	FlavorID  string
	NodeID    string
	State     InstanceState
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (q *Queries) CreateInstance(ctx context.Context, arg CreateInstanceParams) error {
	_, err := q.db.Exec(ctx, createInstance,
		arg.ID,
		arg.ChunkID,
		arg.FlavorID,
		arg.NodeID,
		arg.State,
		arg.CreatedAt,
		arg.UpdatedAt,
	)
	return err
}

const flavorNameExists = `-- name: FlavorNameExists :one
SELECT EXISTS(
    SELECT 1 FROM flavors
    WHERE name = $1 AND chunk_id = $2
)
`

type FlavorNameExistsParams struct {
	Name    string
	ChunkID string
}

func (q *Queries) FlavorNameExists(ctx context.Context, arg FlavorNameExistsParams) (bool, error) {
	row := q.db.QueryRow(ctx, flavorNameExists, arg.Name, arg.ChunkID)
	var exists bool
	err := row.Scan(&exists)
	return exists, err
}

const flavorVersionByHash = `-- name: FlavorVersionByHash :one
SELECT version FROM flavor_versions WHERE hash = $1
`

func (q *Queries) FlavorVersionByHash(ctx context.Context, hash string) (string, error) {
	row := q.db.QueryRow(ctx, flavorVersionByHash, hash)
	var version string
	err := row.Scan(&version)
	return version, err
}

const flavorVersionByID = `-- name: FlavorVersionByID :many
SELECT id, flavor_id, hash, change_hash, build_status, version, files_uploaded, prev_version_id, v.created_at, flavor_version_id, file_hash, file_path, f.created_at FROM flavor_versions v
    JOIN flavor_version_files f ON f.flavor_version_id = v.id
WHERE id = $1
`

type FlavorVersionByIDRow struct {
	ID              string
	FlavorID        string
	Hash            string
	ChangeHash      string
	BuildStatus     BuildStatus
	Version         string
	FilesUploaded   bool
	PrevVersionID   *string
	CreatedAt       time.Time
	FlavorVersionID string
	FileHash        pgtype.Text
	FilePath        string
	CreatedAt_2     time.Time
}

func (q *Queries) FlavorVersionByID(ctx context.Context, id string) ([]FlavorVersionByIDRow, error) {
	rows, err := q.db.Query(ctx, flavorVersionByID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []FlavorVersionByIDRow
	for rows.Next() {
		var i FlavorVersionByIDRow
		if err := rows.Scan(
			&i.ID,
			&i.FlavorID,
			&i.Hash,
			&i.ChangeHash,
			&i.BuildStatus,
			&i.Version,
			&i.FilesUploaded,
			&i.PrevVersionID,
			&i.CreatedAt,
			&i.FlavorVersionID,
			&i.FileHash,
			&i.FilePath,
			&i.CreatedAt_2,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const flavorVersionExists = `-- name: FlavorVersionExists :one
SELECT EXISTS(
    SELECT 1 FROM flavor_versions
    WHERE version = $1 AND flavor_id = $2
)
`

type FlavorVersionExistsParams struct {
	Version  string
	FlavorID string
}

func (q *Queries) FlavorVersionExists(ctx context.Context, arg FlavorVersionExistsParams) (bool, error) {
	row := q.db.QueryRow(ctx, flavorVersionExists, arg.Version, arg.FlavorID)
	var exists bool
	err := row.Scan(&exists)
	return exists, err
}

const flavorVersionFileHashes = `-- name: FlavorVersionFileHashes :many
SELECT flavor_version_id, file_hash, file_path, created_at FROM flavor_version_files WHERE flavor_version_id = $1
`

func (q *Queries) FlavorVersionFileHashes(ctx context.Context, flavorVersionID string) ([]FlavorVersionFile, error) {
	rows, err := q.db.Query(ctx, flavorVersionFileHashes, flavorVersionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []FlavorVersionFile
	for rows.Next() {
		var i FlavorVersionFile
		if err := rows.Scan(
			&i.FlavorVersionID,
			&i.FileHash,
			&i.FilePath,
			&i.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const flavorVersionHashByID = `-- name: FlavorVersionHashByID :one
SELECT hash FROM flavor_versions WHERE id = $1
`

func (q *Queries) FlavorVersionHashByID(ctx context.Context, id string) (string, error) {
	row := q.db.QueryRow(ctx, flavorVersionHashByID, id)
	var hash string
	err := row.Scan(&hash)
	return hash, err
}

const getChunkByID = `-- name: GetChunkByID :many
SELECT c.id, c.name, description, tags, c.created_at, c.updated_at, f.id, chunk_id, f.name, f.created_at, f.updated_at, v.id, flavor_id, hash, change_hash, build_status, version, files_uploaded, prev_version_id, v.created_at, flavor_version_id, file_hash, file_path, vf.created_at FROM chunks c
    LEFT JOIN flavors f ON f.chunk_id = c.id
    LEFT JOIN flavor_versions v ON v.flavor_id = f.id
    LEFT JOIN flavor_version_files vf ON vf.flavor_version_id = v.id
WHERE c.id = $1
`

type GetChunkByIDRow struct {
	ID              string
	Name            string
	Description     string
	Tags            []string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ID_2            *string
	ChunkID         *string
	Name_2          pgtype.Text
	CreatedAt_2     pgtype.Timestamptz
	UpdatedAt_2     pgtype.Timestamptz
	ID_3            *string
	FlavorID        *string
	Hash            pgtype.Text
	ChangeHash      pgtype.Text
	BuildStatus     NullBuildStatus
	Version         pgtype.Text
	FilesUploaded   pgtype.Bool
	PrevVersionID   *string
	CreatedAt_3     pgtype.Timestamptz
	FlavorVersionID *string
	FileHash        pgtype.Text
	FilePath        pgtype.Text
	CreatedAt_4     pgtype.Timestamptz
}

// TODO: read multiple
func (q *Queries) GetChunkByID(ctx context.Context, id string) ([]GetChunkByIDRow, error) {
	rows, err := q.db.Query(ctx, getChunkByID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetChunkByIDRow
	for rows.Next() {
		var i GetChunkByIDRow
		if err := rows.Scan(
			&i.ID,
			&i.Name,
			&i.Description,
			&i.Tags,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.ID_2,
			&i.ChunkID,
			&i.Name_2,
			&i.CreatedAt_2,
			&i.UpdatedAt_2,
			&i.ID_3,
			&i.FlavorID,
			&i.Hash,
			&i.ChangeHash,
			&i.BuildStatus,
			&i.Version,
			&i.FilesUploaded,
			&i.PrevVersionID,
			&i.CreatedAt_3,
			&i.FlavorVersionID,
			&i.FileHash,
			&i.FilePath,
			&i.CreatedAt_4,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getInstance = `-- name: GetInstance :many
SELECT i.id, i.chunk_id, flavor_id, node_id, port, state, i.created_at, i.updated_at, f.id, f.chunk_id, f.name, f.created_at, f.updated_at, c.id, c.name, description, tags, c.created_at, c.updated_at, n.id, address, n.created_at FROM instances i
    JOIN flavors f ON i.chunk_id = f.chunk_id
    JOIN chunks c ON f.chunk_id = c.id
    JOIN nodes n ON i.node_id = n.id
WHERE i.id = $1
`

type GetInstanceRow struct {
	ID          string
	ChunkID     string
	FlavorID    string
	NodeID      string
	Port        *int32
	State       InstanceState
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ID_2        string
	ChunkID_2   string
	Name        string
	CreatedAt_2 time.Time
	UpdatedAt_2 time.Time
	ID_3        string
	Name_2      string
	Description string
	Tags        []string
	CreatedAt_3 time.Time
	UpdatedAt_3 time.Time
	ID_4        string
	Address     netip.Addr
	CreatedAt_4 time.Time
}

func (q *Queries) GetInstance(ctx context.Context, id string) ([]GetInstanceRow, error) {
	rows, err := q.db.Query(ctx, getInstance, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetInstanceRow
	for rows.Next() {
		var i GetInstanceRow
		if err := rows.Scan(
			&i.ID,
			&i.ChunkID,
			&i.FlavorID,
			&i.NodeID,
			&i.Port,
			&i.State,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.ID_2,
			&i.ChunkID_2,
			&i.Name,
			&i.CreatedAt_2,
			&i.UpdatedAt_2,
			&i.ID_3,
			&i.Name_2,
			&i.Description,
			&i.Tags,
			&i.CreatedAt_3,
			&i.UpdatedAt_3,
			&i.ID_4,
			&i.Address,
			&i.CreatedAt_4,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getInstancesByNodeID = `-- name: GetInstancesByNodeID :many
SELECT i.id, i.chunk_id, flavor_id, node_id, port, state, i.created_at, i.updated_at, f.id, f.chunk_id, f.name, f.created_at, f.updated_at, c.id, c.name, description, tags, c.created_at, c.updated_at, n.id, address, n.created_at FROM instances i
    JOIN flavors f ON i.flavor_id = f.id
    JOIN chunks c ON f.chunk_id = c.id
    JOIN nodes n ON i.node_id = n.id
WHERE i.node_id = $1
`

type GetInstancesByNodeIDRow struct {
	ID          string
	ChunkID     string
	FlavorID    string
	NodeID      string
	Port        *int32
	State       InstanceState
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ID_2        string
	ChunkID_2   string
	Name        string
	CreatedAt_2 time.Time
	UpdatedAt_2 time.Time
	ID_3        string
	Name_2      string
	Description string
	Tags        []string
	CreatedAt_3 time.Time
	UpdatedAt_3 time.Time
	ID_4        string
	Address     netip.Addr
	CreatedAt_4 time.Time
}

func (q *Queries) GetInstancesByNodeID(ctx context.Context, nodeID string) ([]GetInstancesByNodeIDRow, error) {
	rows, err := q.db.Query(ctx, getInstancesByNodeID, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetInstancesByNodeIDRow
	for rows.Next() {
		var i GetInstancesByNodeIDRow
		if err := rows.Scan(
			&i.ID,
			&i.ChunkID,
			&i.FlavorID,
			&i.NodeID,
			&i.Port,
			&i.State,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.ID_2,
			&i.ChunkID_2,
			&i.Name,
			&i.CreatedAt_2,
			&i.UpdatedAt_2,
			&i.ID_3,
			&i.Name_2,
			&i.Description,
			&i.Tags,
			&i.CreatedAt_3,
			&i.UpdatedAt_3,
			&i.ID_4,
			&i.Address,
			&i.CreatedAt_4,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const latestFlavorVersionByFlavorID = `-- name: LatestFlavorVersionByFlavorID :one
SELECT id, flavor_id, hash, change_hash, build_status, version, files_uploaded, prev_version_id, created_at FROM flavor_versions WHERE flavor_id = $1
ORDER BY created_at DESC LIMIT 1
`

func (q *Queries) LatestFlavorVersionByFlavorID(ctx context.Context, flavorID string) (FlavorVersion, error) {
	row := q.db.QueryRow(ctx, latestFlavorVersionByFlavorID, flavorID)
	var i FlavorVersion
	err := row.Scan(
		&i.ID,
		&i.FlavorID,
		&i.Hash,
		&i.ChangeHash,
		&i.BuildStatus,
		&i.Version,
		&i.FilesUploaded,
		&i.PrevVersionID,
		&i.CreatedAt,
	)
	return i, err
}

const listChunks = `-- name: ListChunks :many
SELECT c.id, c.name, description, tags, c.created_at, c.updated_at, f.id, chunk_id, f.name, f.created_at, f.updated_at, v.id, flavor_id, hash, change_hash, build_status, version, files_uploaded, prev_version_id, v.created_at, flavor_version_id, file_hash, file_path, vf.created_at FROM chunks c
    LEFT JOIN flavors f ON f.chunk_id = c.id
    LEFT JOIN flavor_versions v ON v.flavor_id = f.id
    LEFT JOIN flavor_version_files vf ON vf.flavor_version_id = v.id
`

type ListChunksRow struct {
	ID              string
	Name            string
	Description     string
	Tags            []string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ID_2            *string
	ChunkID         *string
	Name_2          pgtype.Text
	CreatedAt_2     pgtype.Timestamptz
	UpdatedAt_2     pgtype.Timestamptz
	ID_3            *string
	FlavorID        *string
	Hash            pgtype.Text
	ChangeHash      pgtype.Text
	BuildStatus     NullBuildStatus
	Version         pgtype.Text
	FilesUploaded   pgtype.Bool
	PrevVersionID   *string
	CreatedAt_3     pgtype.Timestamptz
	FlavorVersionID *string
	FileHash        pgtype.Text
	FilePath        pgtype.Text
	CreatedAt_4     pgtype.Timestamptz
}

func (q *Queries) ListChunks(ctx context.Context) ([]ListChunksRow, error) {
	rows, err := q.db.Query(ctx, listChunks)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ListChunksRow
	for rows.Next() {
		var i ListChunksRow
		if err := rows.Scan(
			&i.ID,
			&i.Name,
			&i.Description,
			&i.Tags,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.ID_2,
			&i.ChunkID,
			&i.Name_2,
			&i.CreatedAt_2,
			&i.UpdatedAt_2,
			&i.ID_3,
			&i.FlavorID,
			&i.Hash,
			&i.ChangeHash,
			&i.BuildStatus,
			&i.Version,
			&i.FilesUploaded,
			&i.PrevVersionID,
			&i.CreatedAt_3,
			&i.FlavorVersionID,
			&i.FileHash,
			&i.FilePath,
			&i.CreatedAt_4,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listFlavorsByChunkID = `-- name: ListFlavorsByChunkID :many
SELECT f.id, chunk_id, name, f.created_at, updated_at, v.id, flavor_id, hash, change_hash, build_status, version, files_uploaded, prev_version_id, v.created_at, flavor_version_id, file_hash, file_path, vf.created_at FROM flavors f
    JOIN flavor_versions v ON v.flavor_id = f.id
    JOIN flavor_version_files vf ON vf.flavor_version_id = v.id
WHERE chunk_id = $1
`

type ListFlavorsByChunkIDRow struct {
	ID              string
	ChunkID         string
	Name            string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ID_2            string
	FlavorID        string
	Hash            string
	ChangeHash      string
	BuildStatus     BuildStatus
	Version         string
	FilesUploaded   bool
	PrevVersionID   *string
	CreatedAt_2     time.Time
	FlavorVersionID string
	FileHash        pgtype.Text
	FilePath        string
	CreatedAt_3     time.Time
}

func (q *Queries) ListFlavorsByChunkID(ctx context.Context, chunkID string) ([]ListFlavorsByChunkIDRow, error) {
	rows, err := q.db.Query(ctx, listFlavorsByChunkID, chunkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ListFlavorsByChunkIDRow
	for rows.Next() {
		var i ListFlavorsByChunkIDRow
		if err := rows.Scan(
			&i.ID,
			&i.ChunkID,
			&i.Name,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.ID_2,
			&i.FlavorID,
			&i.Hash,
			&i.ChangeHash,
			&i.BuildStatus,
			&i.Version,
			&i.FilesUploaded,
			&i.PrevVersionID,
			&i.CreatedAt_2,
			&i.FlavorVersionID,
			&i.FileHash,
			&i.FilePath,
			&i.CreatedAt_3,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listInstances = `-- name: ListInstances :many
SELECT i.id, i.chunk_id, flavor_id, node_id, port, state, i.created_at, i.updated_at, f.id, f.chunk_id, f.name, f.created_at, f.updated_at, c.id, c.name, description, tags, c.created_at, c.updated_at, n.id, address, n.created_at FROM instances i
    JOIN flavors f ON i.chunk_id = f.chunk_id
    JOIN chunks c ON f.chunk_id = c.id
    JOIN nodes n ON i.node_id = n.id
`

type ListInstancesRow struct {
	ID          string
	ChunkID     string
	FlavorID    string
	NodeID      string
	Port        *int32
	State       InstanceState
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ID_2        string
	ChunkID_2   string
	Name        string
	CreatedAt_2 time.Time
	UpdatedAt_2 time.Time
	ID_3        string
	Name_2      string
	Description string
	Tags        []string
	CreatedAt_3 time.Time
	UpdatedAt_3 time.Time
	ID_4        string
	Address     netip.Addr
	CreatedAt_4 time.Time
}

func (q *Queries) ListInstances(ctx context.Context) ([]ListInstancesRow, error) {
	rows, err := q.db.Query(ctx, listInstances)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ListInstancesRow
	for rows.Next() {
		var i ListInstancesRow
		if err := rows.Scan(
			&i.ID,
			&i.ChunkID,
			&i.FlavorID,
			&i.NodeID,
			&i.Port,
			&i.State,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.ID_2,
			&i.ChunkID_2,
			&i.Name,
			&i.CreatedAt_2,
			&i.UpdatedAt_2,
			&i.ID_3,
			&i.Name_2,
			&i.Description,
			&i.Tags,
			&i.CreatedAt_3,
			&i.UpdatedAt_3,
			&i.ID_4,
			&i.Address,
			&i.CreatedAt_4,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const markFlavorVersionFilesUploaded = `-- name: MarkFlavorVersionFilesUploaded :exec
UPDATE flavor_versions SET files_uploaded = TRUE WHERE id = $1
`

func (q *Queries) MarkFlavorVersionFilesUploaded(ctx context.Context, id string) error {
	_, err := q.db.Exec(ctx, markFlavorVersionFilesUploaded, id)
	return err
}

const updateChunk = `-- name: UpdateChunk :exec
UPDATE chunks
SET
    name = $1,
    description = $2,
    tags = $3,
    updated_at = now()
WHERE id = $4
`

type UpdateChunkParams struct {
	Name        string
	Description string
	Tags        []string
	ID          string
}

func (q *Queries) UpdateChunk(ctx context.Context, arg UpdateChunkParams) error {
	_, err := q.db.Exec(ctx, updateChunk,
		arg.Name,
		arg.Description,
		arg.Tags,
		arg.ID,
	)
	return err
}

const updateFlavorVersionBuildStatus = `-- name: UpdateFlavorVersionBuildStatus :exec
UPDATE flavor_versions SET build_status = $1 WHERE id = $2
`

type UpdateFlavorVersionBuildStatusParams struct {
	BuildStatus BuildStatus
	ID          string
}

func (q *Queries) UpdateFlavorVersionBuildStatus(ctx context.Context, arg UpdateFlavorVersionBuildStatusParams) error {
	_, err := q.db.Exec(ctx, updateFlavorVersionBuildStatus, arg.BuildStatus, arg.ID)
	return err
}
