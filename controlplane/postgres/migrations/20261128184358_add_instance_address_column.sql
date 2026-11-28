-- migrate:up
ALTER TABLE instances ADD COLUMN address TEXT;

-- migrate:down

