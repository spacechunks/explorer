-- migrate:up
CREATE TABLE IF NOT EXISTS chunks (
    id          UUID             NOT NULL PRIMARY KEY,
    name        VARCHAR(50)      NOT NULL,
    description VARCHAR(100)     NOT NULL,
    tags        VARCHAR(25)[]    NOT NULL,
--  owner       UUID             NOT NULL,
    created_at  TIMESTAMPTZ      NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ      NOT NULL DEFAULT now()
);

--
-- flavors
--

CREATE TYPE build_status AS ENUM (
    'PENDING',
    'IMAGE_BUILD',
    'IMAGE_BUILD_FAILED',
    'CHECKPOINT_BUILD',
    'CHECKPOINT_BUILD_FAILED',
    'COMPLETED'
);

CREATE TABLE IF NOT EXISTS flavors (
    id                   UUID PRIMARY KEY NOT NULL,
    chunk_id             UUID             NOT NULL REFERENCES chunks(id),
    name                 VARCHAR(25)      NOT NULL,
    created_at           TIMESTAMPTZ      NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ      NOT NULL DEFAULT now(),
    UNIQUE (id, name)
);

CREATE TABLE IF NOT EXISTS flavor_versions (
    id              UUID          NOT NULL PRIMARY KEY,
    flavor_id       UUID          NOT NULL REFERENCES flavors(id) ON DELETE CASCADE,
    hash            CHAR(16)      NOT NULL,

    -- hash of all changed and new files. this is needed, so we can detect
    -- if the changed and new files that are sent when uploading flavor files
    -- are the same that have been used for creating the flavor version in the
    -- first place. this also detects if files are missing.
    change_hash     CHAR(16)      NOT NULL,

    build_status    build_status  NOT NULL DEFAULT 'PENDING',
    version         VARCHAR(25)   NOT NULL,
    files_uploaded  BOOL          NOT NULL DEFAULT FALSE,
    prev_version_id UUID,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    UNIQUE (flavor_id, version)
);

CREATE TABLE IF NOT EXISTS flavor_version_files (
    flavor_version_id UUID          NOT NULL REFERENCES flavor_versions(id) ON DELETE CASCADE,
    file_hash         VARCHAR(16)   NOT NULL,
    file_path         VARCHAR(4096) NOT NULL, -- 4096 is PATH_MAX chars on linux
    created_at        TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX flavor_version_idx ON flavor_versions(version);
CREATE INDEX flavor_hash_idx ON flavor_versions(hash);

--
-- instances
--

CREATE TYPE instance_state AS ENUM ('PENDING', 'CREATING', 'RUNNING', 'DELETING', 'DELETED', 'CREATION_FAILED');

CREATE TABLE IF NOT EXISTS nodes (
    id                      UUID         NOT NULL PRIMARY KEY,
    name                    TEXT         NOT NULL,
    address                 INET         NOT NULL,
    checkpoint_api_endpoint TEXT         NOT NULL,
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS instances (
    id                     UUID           NOT NULL PRIMARY KEY,
    chunk_id               UUID           NOT NULL REFERENCES chunks(id),
    flavor_version_id      UUID           NOT NULL REFERENCES flavor_versions(id),
    node_id                UUID           NOT NULL REFERENCES nodes(id),
    port                   INT,
    state                  instance_state NOT NULL DEFAULT 'PENDING',
    created_at             TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ    NOT NULL DEFAULT now()
);

--
-- blob store
--

CREATE TABLE IF NOT EXISTS blobs (
    hash VARCHAR(16)          NOT NULL PRIMARY KEY,
    data BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- migrate:down