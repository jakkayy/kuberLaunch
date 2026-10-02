-- +goose Up
-- Enum order matters: MAX(role) picks the strongest role.
CREATE TYPE member_role AS ENUM ('viewer', 'maintainer', 'owner');

CREATE TABLE teams (
    id         TEXT PRIMARY KEY DEFAULT encode(gen_random_bytes(8), 'hex'),
    slug       TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE team_members (
    team_id    TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       member_role NOT NULL DEFAULT 'viewer',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (team_id, user_id)
);

-- A project member is either a single user or a whole team.
CREATE TABLE project_members (
    id         TEXT PRIMARY KEY DEFAULT encode(gen_random_bytes(8), 'hex'),
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    TEXT REFERENCES users(id) ON DELETE CASCADE,
    team_id    TEXT REFERENCES teams(id) ON DELETE CASCADE,
    role       member_role NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK ((user_id IS NULL) <> (team_id IS NULL)),
    UNIQUE (project_id, user_id),
    UNIQUE (project_id, team_id)
);

CREATE INDEX idx_project_members_user ON project_members(user_id);
CREATE INDEX idx_project_members_team ON project_members(team_id);
CREATE INDEX idx_team_members_user ON team_members(user_id);

-- +goose Down
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
DROP TYPE IF EXISTS member_role;
