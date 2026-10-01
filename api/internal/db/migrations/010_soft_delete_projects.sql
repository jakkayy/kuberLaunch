-- +goose Up
-- Deleted projects are kept (and their slug stays taken, since the archived
-- GitHub repo still uses the name) so history and audit context survive.
ALTER TABLE projects ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE projects DROP COLUMN IF EXISTS deleted_at;
