package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jakkayy/kuberlauncher/api/internal/model"
)

// ErrPromotionPending is returned when the target environment already has an
// open promotion request.
var ErrPromotionPending = errors.New("a promotion to this environment is already awaiting approval")

type PromotionRepository struct {
	db *sql.DB
}

func NewPromotionRepository(db *sql.DB) *PromotionRepository {
	return &PromotionRepository{db: db}
}

const promotionSelect = `SELECT pr.id, pr.project_id, pr.from_env, pr.to_env, pr.image_tag, pr.status, pr.message,
	pr.requested_by, ru.login, COALESCE(pr.decided_by, ''), COALESCE(du.login, ''),
	COALESCE(pr.deployment_id, ''), pr.created_at, pr.decided_at
	FROM promotions pr
	JOIN users ru ON ru.id = pr.requested_by
	LEFT JOIN users du ON du.id = pr.decided_by`

func scanPromotion(row interface{ Scan(...any) error }) (*model.Promotion, error) {
	var p model.Promotion
	err := row.Scan(&p.ID, &p.ProjectID, &p.FromEnv, &p.ToEnv, &p.ImageTag, &p.Status, &p.Message,
		&p.RequestedBy, &p.RequestedByLogin, &p.DecidedBy, &p.DecidedByLogin,
		&p.DeploymentID, &p.CreatedAt, &p.DecidedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PromotionRepository) Create(ctx context.Context, p *model.Promotion) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO promotions (project_id, from_env, to_env, image_tag, status, requested_by)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`,
		p.ProjectID, p.FromEnv, p.ToEnv, p.ImageTag, p.Status, p.RequestedBy,
	).Scan(&p.ID, &p.CreatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrPromotionPending
	}
	return err
}

func (r *PromotionRepository) Find(ctx context.Context, projectID, id string) (*model.Promotion, error) {
	p, err := scanPromotion(r.db.QueryRowContext(ctx, promotionSelect+` WHERE pr.project_id = $1 AND pr.id = $2`, projectID, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (r *PromotionRepository) ListByProject(ctx context.Context, projectID string) ([]model.Promotion, error) {
	rows, err := r.db.QueryContext(ctx, promotionSelect+` WHERE pr.project_id = $1 ORDER BY pr.created_at DESC LIMIT 20`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Promotion
	for rows.Next() {
		p, err := scanPromotion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// Decide moves a pending promotion to status. It returns false if the
// promotion was no longer pending (someone else decided first).
func (r *PromotionRepository) Decide(ctx context.Context, id, userID string, status model.PromotionStatus, message, deploymentID string) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE promotions SET status = $1, decided_by = NULLIF($2, ''), message = $3,
		        deployment_id = NULLIF($4, ''), decided_at = NOW()
		 WHERE id = $5 AND status = 'pending_approval'`,
		status, userID, message, deploymentID, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// Record stores a promotion that was applied immediately (no approval step).
func (r *PromotionRepository) Record(ctx context.Context, p *model.Promotion) error {
	return r.db.QueryRowContext(ctx,
		`INSERT INTO promotions (project_id, from_env, to_env, image_tag, status, message,
		                         requested_by, decided_by, deployment_id, decided_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $7, NULLIF($8, ''), NOW()) RETURNING id, created_at`,
		p.ProjectID, p.FromEnv, p.ToEnv, p.ImageTag, p.Status, p.Message, p.RequestedBy, p.DeploymentID,
	).Scan(&p.ID, &p.CreatedAt)
}

// SetResult records the outcome of applying an approved promotion.
func (r *PromotionRepository) SetResult(ctx context.Context, id string, status model.PromotionStatus, message, deploymentID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE promotions SET status = $1, message = $2, deployment_id = NULLIF($3, '') WHERE id = $4`,
		status, message, deploymentID, id)
	return err
}
