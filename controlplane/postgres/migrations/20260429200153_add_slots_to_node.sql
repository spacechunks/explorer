-- migrate:up
ALTER TABLE nodes
ADD COLUMN slots INTEGER NOT NULL DEFAULT 1;

-- migrate:down
