-- migrate:up
CREATE UNIQUE INDEX idp_id_unique_idx ON users (idp_id);

-- migrate:down

