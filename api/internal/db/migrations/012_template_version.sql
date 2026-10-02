-- +goose Up
-- Existing projects were generated before versioning (version 1).
ALTER TABLE projects ADD COLUMN IF NOT EXISTS template_version INT NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE projects DROP COLUMN IF EXISTS template_version;
