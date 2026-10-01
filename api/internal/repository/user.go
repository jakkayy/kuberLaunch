package repository

import (
	"context"
	"database/sql"

	"github.com/jakkayy/kuberlauncher/api/internal/model"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Upsert inserts or refreshes a user identified by GitHub ID and fills in u.ID.
func (r *UserRepository) Upsert(ctx context.Context, u *model.User) error {
	return r.db.QueryRowContext(ctx,
		`INSERT INTO users (github_id, login, name, avatar_url)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (github_id) DO UPDATE
		   SET login = EXCLUDED.login, name = EXCLUDED.name,
		       avatar_url = EXCLUDED.avatar_url, last_seen_at = NOW()
		 RETURNING id, created_at, last_seen_at`,
		u.GitHubID, u.Login, u.Name, u.AvatarURL,
	).Scan(&u.ID, &u.CreatedAt, &u.LastSeenAt)
}

func (r *UserRepository) FindByLogin(ctx context.Context, login string) (*model.User, error) {
	var u model.User
	err := r.db.QueryRowContext(ctx,
		`SELECT id, github_id, login, name, avatar_url, created_at, last_seen_at
		 FROM users WHERE login = $1`, login,
	).Scan(&u.ID, &u.GitHubID, &u.Login, &u.Name, &u.AvatarURL, &u.CreatedAt, &u.LastSeenAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &u, err
}
