-- +goose Up
ALTER TABLE projects ADD COLUMN IF NOT EXISTS argocd_app TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE projects DROP COLUMN IF EXISTS argocd_app;
