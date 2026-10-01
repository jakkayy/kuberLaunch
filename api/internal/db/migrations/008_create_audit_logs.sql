-- +goose Up
-- project_id has no foreign key so the trail survives project deletion.
CREATE TABLE audit_logs (
    id          BIGSERIAL PRIMARY KEY,
    actor_id    TEXT REFERENCES users(id) ON DELETE SET NULL,
    actor_login TEXT NOT NULL,
    action      TEXT NOT NULL,
    project_id  TEXT,
    status      INT NOT NULL,
    details     JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_project ON audit_logs(project_id, created_at DESC);
CREATE INDEX idx_audit_logs_created ON audit_logs(created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS audit_logs;
