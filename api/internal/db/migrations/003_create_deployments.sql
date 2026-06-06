-- +goose Up
CREATE TYPE deployment_status AS ENUM ('pending', 'building', 'deploying', 'success', 'failed');

CREATE TABLE deployments (
  id              TEXT PRIMARY KEY DEFAULT encode(gen_random_bytes(8), 'hex'),
  project_id      TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  branch          TEXT NOT NULL DEFAULT 'main',
  workflow_run_id BIGINT,
  image           TEXT NOT NULL DEFAULT '',
  triggered_by    TEXT NOT NULL DEFAULT 'user',
  status          deployment_status NOT NULL DEFAULT 'pending',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_deployments_project_id ON deployments(project_id);

-- +goose Down
DROP TABLE IF EXISTS deployments;
DROP TYPE IF EXISTS deployment_status;
