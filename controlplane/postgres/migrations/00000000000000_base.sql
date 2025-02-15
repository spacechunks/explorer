-- migrate:up
CREATE TABLE IF NOT EXISTS chunks (
    id          UUID PRIMARY KEY NOT NULL,
    name        CHAR(25)         NOT NULL,
    description CHAR(50)         NOT NULL,
    tags        CHAR(25)[]       NOT NULL,
--  owner       UUID             NOT NULL,
    created_at  TIMESTAMPTZ      NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ      NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS workloads (
    id UUID NOT NULL PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);

-- migrate:down
