-- +goose Up
ALTER TABLE projects ADD COLUMN IF NOT EXISTS grafana_url TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE projects DROP COLUMN IF EXISTS grafana_url;
