-- migrate:up
CREATE TABLE IF NOT EXISTS users (
    id         UUID        PRIMARY KEY,
    nickname   VARCHAR(16) UNIQUE NOT NULL,
    email      VARCHAR     UNIQUE NOT NULL,
    created_at TIMESTAMPTZ        NOT NULL,
    updated_at TIMESTAMPTZ        NOT NULL
);

ALTER TABLE chunks ADD COLUMN owner UUID references users(id);
ALTER TABLE instances ADD COLUMN owner UUID references users(id);

-- migrate:down

