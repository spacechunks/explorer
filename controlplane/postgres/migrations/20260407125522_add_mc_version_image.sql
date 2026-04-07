-- migrate:up
ALTER TABLE minecraft_versions ADD COLUMN image_url VARCHAR(256);

-- migrate:down

