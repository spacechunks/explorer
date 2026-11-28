-- migrate:up
ALTER TABLE instances DROP COLUMN node_id;

-- migrate:down

