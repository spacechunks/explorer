-- migrate:up
ALTER TABLE flavors ADD COLUMN deleted_at TIMESTAMPTZ;

-- migrate:down

