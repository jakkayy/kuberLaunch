-- +goose Up
-- At most one in-flight deployment per project, enforced across API replicas.
UPDATE deployments SET status = 'failed', updated_at = NOW()
WHERE status IN ('pending', 'building', 'deploying')
  AND id NOT IN (
    SELECT DISTINCT ON (project_id) id FROM deployments
    WHERE status IN ('pending', 'building', 'deploying')
    ORDER BY project_id, created_at DESC
  );
CREATE UNIQUE INDEX uniq_deployments_active_per_project ON deployments(project_id)
WHERE status IN ('pending', 'building', 'deploying');

-- +goose Down
DROP INDEX IF EXISTS uniq_deployments_active_per_project;
