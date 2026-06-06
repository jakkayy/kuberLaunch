package repository

import (
	"context"
	"database/sql"

	"github.com/jakkayy/kuberlauncher/api/internal/model"
)

type DeploymentRepository struct {
	db *sql.DB
}

func NewDeploymentRepository(db *sql.DB) *DeploymentRepository {
	return &DeploymentRepository{db: db}
}

func (r *DeploymentRepository) Create(ctx context.Context, d *model.Deployment) error {
	return r.db.QueryRowContext(ctx,
		`INSERT INTO deployments (project_id, branch, triggered_by, status)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at, updated_at`,
		d.ProjectID, d.Branch, d.TriggeredBy, d.Status,
	).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
}

func (r *DeploymentRepository) FindByID(ctx context.Context, id string) (*model.Deployment, error) {
	var d model.Deployment
	err := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, branch, workflow_run_id, image, triggered_by, status, created_at, updated_at
		 FROM deployments WHERE id = $1`, id,
	).Scan(&d.ID, &d.ProjectID, &d.Branch, &d.WorkflowRunID,
		&d.Image, &d.TriggeredBy, &d.Status, &d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &d, err
}

func (r *DeploymentRepository) FindByProject(ctx context.Context, projectID string) ([]model.Deployment, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, branch, workflow_run_id, image, triggered_by, status, created_at, updated_at
		 FROM deployments WHERE project_id = $1 ORDER BY created_at DESC LIMIT 20`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deployments []model.Deployment
	for rows.Next() {
		var d model.Deployment
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.Branch, &d.WorkflowRunID,
			&d.Image, &d.TriggeredBy, &d.Status, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		deployments = append(deployments, d)
	}
	return deployments, rows.Err()
}

func (r *DeploymentRepository) UpdateStatus(ctx context.Context, id string, status model.DeploymentStatus) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE deployments SET status = $1, updated_at = NOW() WHERE id = $2`, status, id)
	return err
}

func (r *DeploymentRepository) SetWorkflowRunID(ctx context.Context, id string, runID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE deployments SET workflow_run_id = $1, updated_at = NOW() WHERE id = $2`, runID, id)
	return err
}

func (r *DeploymentRepository) SetImage(ctx context.Context, id, image string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE deployments SET image = $1, updated_at = NOW() WHERE id = $2`, image, id)
	return err
}
