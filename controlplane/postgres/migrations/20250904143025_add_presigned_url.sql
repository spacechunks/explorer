-- migrate:up
ALTER TABLE flavor_versions ADD COLUMN presigned_url_expiry_date TIMESTAMPTZ;
ALTER TABLE flavor_versions ADD COLUMN presigned_url VARCHAR;

-- migrate:down
