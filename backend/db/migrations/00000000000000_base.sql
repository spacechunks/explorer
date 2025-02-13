-- migrate:up
CREATE TABLE IF NOT EXISTS chunks (
    id UUID NOT NULL PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS workloads (
    id UUID NOT NULL PRIMARY KEY
);

-- migrate:down
