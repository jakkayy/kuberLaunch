-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TYPE project_runtime AS ENUM ('nextjs', 'nestjs', 'go', 'fastapi');
CREATE TYPE project_database AS ENUM ('postgres', 'mysql', 'mongodb', 'none');
CREATE TYPE project_cache AS ENUM ('redis', 'none');
CREATE TYPE project_status AS ENUM ('creating', 'ready', 'failed');

CREATE TABLE projects (
    id         TEXT PRIMARY KEY DEFAULT encode(gen_random_bytes(8), 'hex'),
    name       TEXT NOT NULL,
    slug       TEXT NOT NULL UNIQUE,
    runtime    project_runtime NOT NULL,
    database   project_database NOT NULL DEFAULT 'none',
    cache      project_cache NOT NULL DEFAULT 'none',
    repo_url   TEXT NOT NULL DEFAULT '',
    status     project_status NOT NULL DEFAULT 'creating',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_projects_slug ON projects(slug);
CREATE INDEX idx_projects_status ON projects(status);

-- +goose Down
DROP TABLE IF EXISTS projects;
DROP TYPE IF EXISTS project_status;
DROP TYPE IF EXISTS project_cache;
DROP TYPE IF EXISTS project_database;
DROP TYPE IF EXISTS project_runtime;
