-- migrate:up
CREATE TABLE IF NOT EXISTS minecraft_versions (
    version VARCHAR PRIMARY KEY,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- migrate:down

