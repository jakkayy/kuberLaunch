-- +goose Up
-- The generator never supported MongoDB; remove it from the enum.
UPDATE projects SET database = 'none' WHERE database = 'mongodb';
ALTER TYPE project_database RENAME TO project_database_old;
CREATE TYPE project_database AS ENUM ('postgres', 'mysql', 'none');
ALTER TABLE projects ALTER COLUMN database DROP DEFAULT;
ALTER TABLE projects ALTER COLUMN database TYPE project_database USING database::text::project_database;
ALTER TABLE projects ALTER COLUMN database SET DEFAULT 'none';
DROP TYPE project_database_old;

-- +goose Down
ALTER TYPE project_database ADD VALUE IF NOT EXISTS 'mongodb';
