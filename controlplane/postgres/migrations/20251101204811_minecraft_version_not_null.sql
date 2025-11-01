-- migrate:up
ALTER TABLE flavor_versions ALTER COLUMN minecraft_version SET NOT NULL

-- migrate:down

