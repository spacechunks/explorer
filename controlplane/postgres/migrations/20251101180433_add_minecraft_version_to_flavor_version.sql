-- migrate:up
ALTER TABLE flavor_versions ADD COLUMN minecraft_version VARCHAR REFERENCES minecraft_versions(version)

-- migrate:down
´
