-- +goose Up
CREATE TABLE users (
    id           TEXT PRIMARY KEY DEFAULT encode(gen_random_bytes(8), 'hex'),
    github_id    BIGINT NOT NULL UNIQUE,
    login        TEXT NOT NULL UNIQUE,
    name         TEXT NOT NULL DEFAULT '',
    avatar_url   TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS users;
