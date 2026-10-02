package repository

import (
	"context"
	"database/sql"

	"github.com/jakkayy/kuberlauncher/api/internal/model"
)

type AccessRepository struct {
	db *sql.DB
}

func NewAccessRepository(db *sql.DB) *AccessRepository {
	return &AccessRepository{db: db}
}

// ProjectRole returns the strongest role userID holds on projectID, directly
// or through any team, or RoleNone.
func (r *AccessRepository) ProjectRole(ctx context.Context, projectID, userID string) (model.Role, error) {
	var role sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT MAX(pm.role)::text
		 FROM project_members pm
		 LEFT JOIN team_members tm ON tm.team_id = pm.team_id
		 WHERE pm.project_id = $1 AND (pm.user_id = $2 OR tm.user_id = $2)`,
		projectID, userID,
	).Scan(&role)
	if err != nil {
		return model.RoleNone, err
	}
	return model.Role(role.String), nil
}

// ProjectIDsForUser returns every project the user can see.
func (r *AccessRepository) ProjectIDsForUser(ctx context.Context, userID string) (map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT pm.project_id
		 FROM project_members pm
		 LEFT JOIN team_members tm ON tm.team_id = pm.team_id
		 WHERE pm.user_id = $1 OR tm.user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

// UpsertProjectMember grants role to exactly one of userID or teamID.
func (r *AccessRepository) UpsertProjectMember(ctx context.Context, projectID, userID, teamID string, role model.Role) error {
	if userID != "" {
		_, err := r.db.ExecContext(ctx,
			`INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, $3)
			 ON CONFLICT (project_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
			projectID, userID, role)
		return err
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO project_members (project_id, team_id, role) VALUES ($1, $2, $3)
		 ON CONFLICT (project_id, team_id) DO UPDATE SET role = EXCLUDED.role`,
		projectID, teamID, role)
	return err
}

func (r *AccessRepository) ListProjectMembers(ctx context.Context, projectID string) ([]model.ProjectMember, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT pm.id, pm.project_id, COALESCE(pm.user_id, ''), COALESCE(u.login, ''),
		        COALESCE(pm.team_id, ''), COALESCE(t.slug, ''), pm.role, pm.created_at
		 FROM project_members pm
		 LEFT JOIN users u ON u.id = pm.user_id
		 LEFT JOIN teams t ON t.id = pm.team_id
		 WHERE pm.project_id = $1
		 ORDER BY pm.role DESC, pm.created_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ProjectMember
	for rows.Next() {
		var m model.ProjectMember
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.UserID, &m.Login, &m.TeamID, &m.TeamSlug, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *AccessRepository) FindProjectMember(ctx context.Context, projectID, memberID string) (*model.ProjectMember, error) {
	var m model.ProjectMember
	err := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, COALESCE(user_id, ''), COALESCE(team_id, ''), role, created_at
		 FROM project_members WHERE project_id = $1 AND id = $2`, projectID, memberID,
	).Scan(&m.ID, &m.ProjectID, &m.UserID, &m.TeamID, &m.Role, &m.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &m, err
}

func (r *AccessRepository) CountDirectOwners(ctx context.Context, projectID string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM project_members WHERE project_id = $1 AND role = 'owner'`, projectID,
	).Scan(&n)
	return n, err
}

func (r *AccessRepository) DeleteProjectMember(ctx context.Context, projectID, memberID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM project_members WHERE project_id = $1 AND id = $2`, projectID, memberID)
	return err
}

func (r *AccessRepository) CreateTeam(ctx context.Context, t *model.Team) error {
	return r.db.QueryRowContext(ctx,
		`INSERT INTO teams (slug, name) VALUES ($1, $2) RETURNING id, created_at`,
		t.Slug, t.Name,
	).Scan(&t.ID, &t.CreatedAt)
}

func (r *AccessRepository) FindTeamBySlug(ctx context.Context, slug string) (*model.Team, error) {
	var t model.Team
	err := r.db.QueryRowContext(ctx,
		`SELECT id, slug, name, created_at FROM teams WHERE slug = $1`, slug,
	).Scan(&t.ID, &t.Slug, &t.Name, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &t, err
}

// ListTeams returns all teams (userID == "") or the teams userID belongs to,
// with MyRole filled for that user.
func (r *AccessRepository) ListTeams(ctx context.Context, userID string) ([]model.Team, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT t.id, t.slug, t.name, COALESCE(tm.role::text, ''), t.created_at
		 FROM teams t
		 LEFT JOIN team_members tm ON tm.team_id = t.id AND tm.user_id = NULLIF($1, '')
		 WHERE $1 = '' OR tm.user_id IS NOT NULL
		 ORDER BY t.slug`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Team
	for rows.Next() {
		var t model.Team
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.MyRole, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *AccessRepository) TeamRole(ctx context.Context, teamID, userID string) (model.Role, error) {
	var role string
	err := r.db.QueryRowContext(ctx,
		`SELECT role FROM team_members WHERE team_id = $1 AND user_id = $2`, teamID, userID,
	).Scan(&role)
	if err == sql.ErrNoRows {
		return model.RoleNone, nil
	}
	return model.Role(role), err
}

func (r *AccessRepository) UpsertTeamMember(ctx context.Context, teamID, userID string, role model.Role) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, $3)
		 ON CONFLICT (team_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		teamID, userID, role)
	return err
}

func (r *AccessRepository) ListTeamMembers(ctx context.Context, teamID string) ([]model.TeamMember, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT u.id, u.login, tm.role FROM team_members tm JOIN users u ON u.id = tm.user_id
		 WHERE tm.team_id = $1 ORDER BY tm.role DESC, u.login`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.TeamMember
	for rows.Next() {
		var m model.TeamMember
		if err := rows.Scan(&m.UserID, &m.Login, &m.Role); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *AccessRepository) DeleteTeamMember(ctx context.Context, teamID, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`, teamID, userID)
	return err
}
