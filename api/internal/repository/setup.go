package repository

import (
	"context"
	"database/sql"
	"time"
)

type SetupStepRow struct {
	Step      string    `json:"step"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SetupRepository struct {
	db *sql.DB
}

func NewSetupRepository(db *sql.DB) *SetupRepository {
	return &SetupRepository{db: db}
}

func (r *SetupRepository) SetStep(ctx context.Context, projectID, step, status, message string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO project_setup_steps (project_id, step, status, message, updated_at)
		 VALUES ($1, $2, $3, $4, NOW())
		 ON CONFLICT (project_id, step) DO UPDATE
		   SET status = EXCLUDED.status, message = EXCLUDED.message, updated_at = NOW()`,
		projectID, step, status, message)
	return err
}

func (r *SetupRepository) Steps(ctx context.Context, projectID string) (map[string]SetupStepRow, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT step, status, message, updated_at FROM project_setup_steps WHERE project_id = $1`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]SetupStepRow{}
	for rows.Next() {
		var s SetupStepRow
		if err := rows.Scan(&s.Step, &s.Status, &s.Message, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out[s.Step] = s
	}
	return out, rows.Err()
}

// TryLock takes a session-level Postgres advisory lock for key on a dedicated
// connection, so only one API replica runs setup for a project at a time.
// Call unlock when done. ok is false if another holder has the lock.
func (r *SetupRepository) TryLock(ctx context.Context, key string) (unlock func(), ok bool, err error) {
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return nil, false, err
	}
	if err := conn.QueryRowContext(ctx,
		`SELECT pg_try_advisory_lock(hashtext('kuberlaunch-setup:' || $1))`, key,
	).Scan(&ok); err != nil {
		conn.Close()
		return nil, false, err
	}
	if !ok {
		conn.Close()
		return nil, false, nil
	}
	return func() {
		_, _ = conn.ExecContext(context.Background(),
			`SELECT pg_advisory_unlock(hashtext('kuberlaunch-setup:' || $1))`, key)
		conn.Close()
	}, true, nil
}
