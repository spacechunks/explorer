-- migrate:up
ALTER TABLE chunks RENAME COLUMN owner TO owner_id;
ALTER TABLE instances RENAME COLUMN owner TO owner_id;

-- migrate:down

