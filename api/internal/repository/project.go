package repository

import (
	"context"
	"database/sql"
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
		INSERT INTO projects (name, slug, runtime, database, cache, status, template_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`
	return r.db.QueryRowContext(ctx, query,
		p.Name, p.Slug, p.Runtime, p.Database, p.Cache, p.Status, p.TemplateVersion,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *ProjectRepository) FindByID(ctx context.Context, id string) (*model.Project, error) {
	var p model.Project
	query := `SELECT id, name, slug, runtime, database, cache, repo_url, argocd_app, grafana_url, status, template_version, created_at, updated_at
	          FROM projects WHERE id = $1 AND deleted_at IS NULL`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.Name, &p.Slug, &p.Runtime, &p.Database,
		&p.Cache, &p.RepoURL, &p.ArgocdApp, &p.GrafanaURL, &p.Status, &p.TemplateVersion, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &p, err
}

func (r *ProjectRepository) FindBySlug(ctx context.Context, slug string) (*model.Project, error) {
	return r.findOne(ctx, "slug", slug)
}

// findOne looks up a live project by a unique column; column is never user input.
func (r *ProjectRepository) findOne(ctx context.Context, column, value string) (*model.Project, error) {
	var p model.Project
	query := `SELECT id, name, slug, runtime, database, cache, repo_url, argocd_app, grafana_url, status, template_version, created_at, updated_at
	          FROM projects WHERE ` + column + ` = $1 AND deleted_at IS NULL`
	err := r.db.QueryRowContext(ctx, query, value).Scan(
		&p.ID, &p.Name, &p.Slug, &p.Runtime, &p.Database,
		&p.Cache, &p.RepoURL, &p.ArgocdApp, &p.GrafanaURL, &p.Status, &p.TemplateVersion, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &p, err
}

func (r *ProjectRepository) FindAll(ctx context.Context) ([]model.Project, error) {
	query := `SELECT id, name, slug, runtime, database, cache, repo_url, argocd_app, grafana_url, status, template_version, created_at, updated_at
	          FROM projects WHERE deleted_at IS NULL ORDER BY created_at DESC`
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
			&p.Cache, &p.RepoURL, &p.ArgocdApp, &p.GrafanaURL, &p.Status, &p.TemplateVersion, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (r *ProjectRepository) SetRepoURL(ctx context.Context, id, repoURL string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE projects SET repo_url = $1, updated_at = NOW() WHERE id = $2`,
		repoURL, id)
	return err
}

func (r *ProjectRepository) SetTemplateVersion(ctx context.Context, id string, version int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE projects SET template_version = $1, updated_at = NOW() WHERE id = $2`, version, id)
	return err
}

func (r *ProjectRepository) SetGrafanaURL(ctx context.Context, id, grafanaURL string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE projects SET grafana_url = $1, updated_at = NOW() WHERE id = $2`,
		grafanaURL, id)
	return err
}

func (r *ProjectRepository) SetArgoCDApp(ctx context.Context, id, argocdApp string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE projects SET argocd_app = $1, updated_at = NOW() WHERE id = $2`,
		argocdApp, id)
	return err
}

func (r *ProjectRepository) UpdateStatus(ctx context.Context, id string, status model.ProjectStatus) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE projects SET status = $1, updated_at = NOW() WHERE id = $2`, status, id)
	return err
}

// Delete permanently removes a project row and cascades to its deployments.
func (r *ProjectRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM projects WHERE id = $1`, id)
	return err
}

// SoftDelete hides a project from all queries while keeping its row.
func (r *ProjectRepository) SoftDelete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE projects SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	return err
}

// SlugExists includes soft-deleted projects: their archived repos keep the name.
func (r *ProjectRepository) SlugExists(ctx context.Context, slug string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM projects WHERE slug = $1)`, slug,
	).Scan(&exists)
	return exists, err
}

// ToSlug converts a project name to a k8s-safe slug: lowercase, spaces and
// underscores become hyphens, other characters are dropped and repeated
// hyphens collapse. The result still needs validate.Slug.
func ToSlug(name string) string {
	var b strings.Builder
	lastHyphen := true // also trims leading hyphens
	for _, r := range strings.ToLower(name) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastHyphen = false
		case r == '-' || r == ' ' || r == '_':
			if !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}
