-- migrate:up
ALTER TABLE instances DROP COLUMN chunk_id;

-- migrate:down
