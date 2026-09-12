-- migrate:up
CREATE TABLE cas_blobs
(
    hash       varchar(16) PRIMARY KEY,
    size_bytes bigint      NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO cas_blobs (hash)
SELECT DISTINCT f.file_hash
FROM flavor_version_files f
         JOIN flavor_versions v ON v.id = f.flavor_version_id
WHERE v.build_status IN ('CHECKPOINT_BUILD', 'CHECKPOINT_BUILD_FAILED', 'COMPLETED') ON CONFLICT (hash) DO NOTHING;

-- migrate:down
DROP TABLE cas_blobs;