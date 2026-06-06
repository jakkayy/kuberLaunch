package model

import "time"

type DeploymentStatus string

const (
	DeploymentStatusPending   DeploymentStatus = "pending"
	DeploymentStatusBuilding  DeploymentStatus = "building"
	DeploymentStatusDeploying DeploymentStatus = "deploying"
	DeploymentStatusSuccess   DeploymentStatus = "success"
	DeploymentStatusFailed    DeploymentStatus = "failed"
)

func (s DeploymentStatus) IsTerminal() bool {
	return s == DeploymentStatusSuccess || s == DeploymentStatusFailed
}

type Deployment struct {
	ID            string           `json:"id"`
	ProjectID     string           `json:"project_id"`
	Branch        string           `json:"branch"`
	WorkflowRunID *int64           `json:"workflow_run_id,omitempty"`
	Image         string           `json:"image,omitempty"`
	TriggeredBy   string           `json:"triggered_by"`
	Status        DeploymentStatus `json:"status"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

type TriggerDeploymentRequest struct {
	Branch string `json:"branch" binding:"required,min=1,max=100"`
}
