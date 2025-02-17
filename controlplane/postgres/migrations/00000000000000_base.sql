-- migrate:up
CREATE TYPE instance_state AS ENUM ('PENDING', 'STARTING', 'RUNNING', 'DELETING', 'DELETED');

CREATE TABLE IF NOT EXISTS chunks (
    id          UUID PRIMARY KEY NOT NULL,
    name        CHAR(25)         NOT NULL,
    description CHAR(50)         NOT NULL,
    tags        CHAR(25)[]       NOT NULL,
--  owner       UUID             NOT NULL,
    created_at  TIMESTAMPTZ      NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ      NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS nodes (
    id         UUID NOT NULL PRIMARY KEY,
    address    INET NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS instances (
    id         UUID           NOT NULL PRIMARY KEY,
    chunk_id   UUID           NOT NULL REFERENCES chunks(id),
    image      TEXT           NOT NULL,
    node_id    UUID           NOT NULL REFERENCES nodes(id),
    state      instance_state NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ    NOT NULL DEFAULT now()
);

-- migrate:down
