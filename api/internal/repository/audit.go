package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/jakkayy/kuberlauncher/api/internal/model"
)

type AuditRepository struct {
	db *sql.DB
}

func NewAuditRepository(db *sql.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) Insert(ctx context.Context, e *model.AuditEntry) error {
	details, err := json.Marshal(e.Details)
	if err != nil {
		return err
	}
	return r.db.QueryRowContext(ctx,
		`INSERT INTO audit_logs (actor_id, actor_login, action, project_id, status, details)
		 VALUES (NULLIF($1, ''), $2, $3, NULLIF($4, ''), $5, $6)
		 RETURNING id, created_at`,
		e.ActorID, e.ActorLogin, e.Action, e.ProjectID, e.Status, details,
	).Scan(&e.ID, &e.CreatedAt)
}

// List returns the newest entries, for one project or (projectID == "") all.
func (r *AuditRepository) List(ctx context.Context, projectID string, limit int) ([]model.AuditEntry, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, COALESCE(actor_id, ''), actor_login, action, COALESCE(project_id, ''), status, details, created_at
		 FROM audit_logs
		 WHERE $1 = '' OR project_id = $1
		 ORDER BY created_at DESC, id DESC
		 LIMIT $2`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.AuditEntry
	for rows.Next() {
		var e model.AuditEntry
		var details []byte
		if err := rows.Scan(&e.ID, &e.ActorID, &e.ActorLogin, &e.Action, &e.ProjectID, &e.Status, &details, &e.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(details, &e.Details); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
