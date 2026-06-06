package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jakkayy/kuberlauncher/api/internal/model"
)

type ProjectRepository struct {
	db *sql.DB
}

func NewProjectRepository(db *sql.DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) Create(ctx context.Context, p *model.Project) error {
	query := `
		INSERT INTO projects (name, slug, runtime, database, cache, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`
	return r.db.QueryRowContext(ctx, query,
		p.Name, p.Slug, p.Runtime, p.Database, p.Cache, p.Status,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *ProjectRepository) FindByID(ctx context.Context, id string) (*model.Project, error) {
	var p model.Project
	query := `SELECT id, name, slug, runtime, database, cache, repo_url, status, created_at, updated_at
	          FROM projects WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.Name, &p.Slug, &p.Runtime, &p.Database,
		&p.Cache, &p.RepoURL, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &p, err
}

func (r *ProjectRepository) FindAll(ctx context.Context) ([]model.Project, error) {
	query := `SELECT id, name, slug, runtime, database, cache, repo_url, status, created_at, updated_at
	          FROM projects ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []model.Project
	for rows.Next() {
		var p model.Project
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Slug, &p.Runtime, &p.Database,
			&p.Cache, &p.RepoURL, &p.Status, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (r *ProjectRepository) UpdateStatus(ctx context.Context, id string, status model.ProjectStatus) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE projects SET status = $1, updated_at = NOW() WHERE id = $2`, status, id)
	return err
}

func (r *ProjectRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM projects WHERE id = $1`, id)
	return err
}

func (r *ProjectRepository) SlugExists(ctx context.Context, slug string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM projects WHERE slug = $1)`, slug,
	).Scan(&exists)
	return exists, err
}

// toSlug converts a project name to a k8s-safe slug.
func ToSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	var b strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return fmt.Sprintf("%.50s", strings.Trim(b.String(), "-"))
}
