-- +goose Up
CREATE TABLE project_setup_steps (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    step       TEXT NOT NULL,
    status     TEXT NOT NULL, -- pending | running | done | failed
    message    TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, step)
);

-- +goose Down
DROP TABLE IF EXISTS project_setup_steps;
