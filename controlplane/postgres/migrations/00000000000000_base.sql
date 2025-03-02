-- migrate:up
CREATE TYPE instance_state AS ENUM ('PENDING', 'STARTING', 'RUNNING', 'DELETING', 'DELETED');

CREATE TABLE IF NOT EXISTS chunks (
    id          UUID             NOT NULL PRIMARY KEY,
    name        VARCHAR(25)      NOT NULL,
    description VARCHAR(50)      NOT NULL,
    tags        VARCHAR(25)[]    NOT NULL,
--  owner       UUID             NOT NULL,
    created_at  TIMESTAMPTZ      NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ      NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS flavors (
    id                   UUID PRIMARY KEY NOT NULL,
    chunk_id             UUID             NOT NULL REFERENCES chunks(id),
    name                 VARCHAR(25)      NOT NULL,
    created_at           TIMESTAMPTZ      NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ      NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS nodes (
    id         UUID        NOT NULL PRIMARY KEY,
    address    INET        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS instances (
    id             UUID           NOT NULL PRIMARY KEY,
    chunk_id       UUID           NOT NULL REFERENCES chunks(id),
    flavor_id      UUID           NOT NULL REFERENCES flavors(id),
    node_id        UUID           NOT NULL REFERENCES nodes(id),
--  port           INT,
    state          instance_state NOT NULL DEFAULT 'PENDING',
    created_at     TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ    NOT NULL DEFAULT now()
);

-- migrate:down
