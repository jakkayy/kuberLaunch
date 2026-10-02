package repository

import (
	"context"
	"database/sql"

	"github.com/jakkayy/kuberlauncher/api/internal/model"
)

type EnvironmentRepository struct {
	db *sql.DB
}

func NewEnvironmentRepository(db *sql.DB) *EnvironmentRepository {
	return &EnvironmentRepository{db: db}
}

const envColumns = `id, project_id, name, namespace, argocd_app, created_at`

func scanEnv(row interface{ Scan(...any) error }) (*model.Environment, error) {
	var e model.Environment
	if err := row.Scan(&e.ID, &e.ProjectID, &e.Name, &e.Namespace, &e.ArgocdApp, &e.CreatedAt); err != nil {
		return nil, err
	}
	return &e, nil
}

// Upsert creates or updates the environment identified by (ProjectID, Name).
func (r *EnvironmentRepository) Upsert(ctx context.Context, e *model.Environment) error {
	return r.db.QueryRowContext(ctx,
		`INSERT INTO environments (project_id, name, namespace, argocd_app)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (project_id, name) DO UPDATE
		   SET namespace = EXCLUDED.namespace, argocd_app = EXCLUDED.argocd_app
		 RETURNING id, created_at`,
		e.ProjectID, e.Name, e.Namespace, e.ArgocdApp,
	).Scan(&e.ID, &e.CreatedAt)
}

// ListByProject returns a project's environments in promotion order.
func (r *EnvironmentRepository) ListByProject(ctx context.Context, projectID string) ([]model.Environment, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+envColumns+` FROM environments WHERE project_id = $1
		 ORDER BY CASE name WHEN 'dev' THEN 1 WHEN 'staging' THEN 2 ELSE 3 END`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Environment
	for rows.Next() {
		e, err := scanEnv(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

func (r *EnvironmentRepository) Find(ctx context.Context, projectID string, name model.EnvName) (*model.Environment, error) {
	e, err := scanEnv(r.db.QueryRowContext(ctx,
		`SELECT `+envColumns+` FROM environments WHERE project_id = $1 AND name = $2`, projectID, name))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return e, err
}

func (r *EnvironmentRepository) FindByArgoApp(ctx context.Context, app string) (*model.Environment, error) {
	e, err := scanEnv(r.db.QueryRowContext(ctx,
		`SELECT `+envColumns+` FROM environments WHERE argocd_app = $1 AND argocd_app <> ''`, app))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return e, err
}
