-- migrate:up
ALTER TABLE instances ADD COLUMN metadata JSONB NOT NULL DEFAULT '{}';

-- migrate:down

