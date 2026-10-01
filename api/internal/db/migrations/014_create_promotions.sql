-- +goose Up
CREATE TABLE promotions (
    id            TEXT PRIMARY KEY DEFAULT encode(gen_random_bytes(8), 'hex'),
    project_id    TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    from_env      TEXT NOT NULL,
    to_env        TEXT NOT NULL,
    image_tag     TEXT NOT NULL,
    -- pending_approval | rejected | applied | failed
    status        TEXT NOT NULL,
    message       TEXT NOT NULL DEFAULT '',
    requested_by  TEXT NOT NULL REFERENCES users(id),
    decided_by    TEXT REFERENCES users(id),
    deployment_id TEXT REFERENCES deployments(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    decided_at    TIMESTAMPTZ
);
CREATE INDEX idx_promotions_project ON promotions(project_id, created_at DESC);
-- One open request per target environment.
CREATE UNIQUE INDEX uniq_promotions_pending ON promotions(project_id, to_env)
WHERE status = 'pending_approval';

-- +goose Down
DROP TABLE IF EXISTS promotions;
