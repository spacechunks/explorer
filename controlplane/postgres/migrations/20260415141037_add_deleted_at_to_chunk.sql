-- migrate:up
ALTER TABLE chunks ADD COLUMN deleted_at TIMESTAMPTZ;

-- migrate:down

