-- migrate:up
ALTER TABLE flavor_versions ADD COLUMN min_players INTEGER NOT NULL DEFAULT 1;
ALTER TABLE flavor_versions ADD COLUMN max_players INTEGER NOT NULL DEFAULT 1;

-- migrate:down
