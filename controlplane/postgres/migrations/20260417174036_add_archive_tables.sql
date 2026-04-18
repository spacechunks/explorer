-- migrate:up
CREATE TABLE chunk_archive(
    id         UUID PRIMARY KEY NOT NULL,
    owner_id   UUID             NOT NULL,
    data       JSONB            NOT NULL,
    created_at TIMESTAMPTZ      NOT NULL
);

CREATE INDEX archived_chunk_owner_idx ON chunk_archive(owner_id);

CREATE TABLE flavor_archive(
    id         UUID PRIMARY KEY NOT NULL,
    chunk_id   UUID             NOT NULL REFERENCES chunk_archive(id),
    data       JSONB            NOT NULL,
    created_at TIMESTAMPTZ      NOT NULL
);

CREATE TABLE flavor_version_archive(
    id         UUID PRIMARY KEY NOT NULL,
    flavor_id  UUID             NOT NULL REFERENCES flavor_archive(id),
    data       JSONB            NOT NULL,
    created_at TIMESTAMPTZ      NOT NULL
);

-- migrate:down

