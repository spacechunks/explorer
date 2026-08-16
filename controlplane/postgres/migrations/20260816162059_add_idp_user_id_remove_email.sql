-- migrate:up
ALTER TABLE users ADD COLUMN idp_id VARCHAR(256) UNIQUE NOT NULL DEFAULT '';
ALTER TABLE users DROP COLUMN email;

-- migrate:down

