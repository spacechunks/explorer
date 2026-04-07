-- migrate:up
ALTER TABLE minecraft_versions ALTER COLUMN image_url SET NOT NULL;

-- migrate:down

