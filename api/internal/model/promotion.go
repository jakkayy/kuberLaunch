package model

import "time"

type PromotionStatus string

const (
	PromotionPending  PromotionStatus = "pending_approval"
	PromotionRejected PromotionStatus = "rejected"
	PromotionApplied  PromotionStatus = "applied"
	PromotionFailed   PromotionStatus = "failed"
)

// Promotion moves the image running in FromEnv to ToEnv without rebuilding.
type Promotion struct {
	ID               string          `json:"id"`
	ProjectID        string          `json:"project_id"`
	FromEnv          EnvName         `json:"from_env"`
	ToEnv            EnvName         `json:"to_env"`
	ImageTag         string          `json:"image_tag"`
	Status           PromotionStatus `json:"status"`
	Message          string          `json:"message,omitempty"`
	RequestedBy      string          `json:"requested_by"`
	RequestedByLogin string          `json:"requested_by_login,omitempty"`
	DecidedBy        string          `json:"decided_by,omitempty"`
	DecidedByLogin   string          `json:"decided_by_login,omitempty"`
	DeploymentID     string          `json:"deployment_id,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	DecidedAt        *time.Time      `json:"decided_at,omitempty"`
}

// PromotionSource is the environment an environment is promoted from.
func PromotionSource(to EnvName) (EnvName, bool) {
	switch to {
	case EnvStaging:
		return EnvDev, true
	case EnvProd:
		return EnvStaging, true
	}
	return "", false
}

// RequiresApproval reports whether promotions into env need a second owner.
func (e EnvName) RequiresApproval() bool { return e == EnvProd }
