-- migrate:up
ALTER TABLE chunks ALTER COLUMN owner SET NOT NULL;
ALTER TABLE instances ALTER COLUMN owner SET NOT NULL;

-- migrate:down

