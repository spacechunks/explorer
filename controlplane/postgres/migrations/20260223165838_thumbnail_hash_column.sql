-- migrate:up
ALTER TABLE chunks ADD COLUMN thumbnail_hash VARCHAR(16);
ALTER TABLE chunks ADD COLUMN thumbnail_updated_at TIMESTAMPTZ DEFAULT now() NOT NULL;

-- migrate:down
