package model

import "time"

type AuditEntry struct {
	ID         int64             `json:"id"`
	ActorID    string            `json:"actor_id,omitempty"`
	ActorLogin string            `json:"actor_login"`
	Action     string            `json:"action"`
	ProjectID  string            `json:"project_id,omitempty"`
	Status     int               `json:"status"`
	Details    map[string]string `json:"details"`
	CreatedAt  time.Time         `json:"created_at"`
}
