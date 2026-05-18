-- migrate:up
ALTER TABLE instances ADD COLUMN ordered_by VARCHAR(100) NOT NULL DEFAULT '';

-- migrate:down
