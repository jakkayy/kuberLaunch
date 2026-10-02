package model

import "time"

type User struct {
	ID         string    `json:"id"`
	GitHubID   int64     `json:"github_id"`
	Login      string    `json:"login"`
	Name       string    `json:"name"`
	AvatarURL  string    `json:"avatar_url"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}
