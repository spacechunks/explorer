-- migrate:up
ALTER TABLE flavor_versions DROP COLUMN change_hash;

-- migrate:down

