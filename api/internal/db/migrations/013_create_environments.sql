-- +goose Up
CREATE TABLE environments (
    id         TEXT PRIMARY KEY DEFAULT encode(gen_random_bytes(8), 'hex'),
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name       TEXT NOT NULL CHECK (name IN ('dev', 'staging', 'prod')),
    namespace  TEXT NOT NULL,
    argocd_app TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, name)
);
CREATE INDEX idx_environments_argocd_app ON environments(argocd_app);

-- Projects registered before environments deployed only to <slug>-dev via
-- an app named <slug>; keep that app as dev until it is re-registered.
INSERT INTO environments (project_id, name, namespace, argocd_app)
SELECT id, 'dev', slug || '-dev', argocd_app FROM projects WHERE argocd_app <> '';

ALTER TABLE deployments ADD COLUMN IF NOT EXISTS environment TEXT NOT NULL DEFAULT 'dev'
    CHECK (environment IN ('dev', 'staging', 'prod'));

-- One in-flight deployment per environment instead of per project.
DROP INDEX IF EXISTS uniq_deployments_active_per_project;
CREATE UNIQUE INDEX uniq_deployments_active_per_env ON deployments(project_id, environment)
WHERE status IN ('pending', 'building', 'deploying');

-- +goose Down
DROP INDEX IF EXISTS uniq_deployments_active_per_env;
CREATE UNIQUE INDEX uniq_deployments_active_per_project ON deployments(project_id)
WHERE status IN ('pending', 'building', 'deploying');
ALTER TABLE deployments DROP COLUMN IF EXISTS environment;
DROP TABLE IF EXISTS environments;
