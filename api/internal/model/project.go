package model

import "time"

type Runtime string
type Database string
type Cache string
type ProjectStatus string

const (
	RuntimeNextJS  Runtime = "nextjs"
	RuntimeNestJS  Runtime = "nestjs"
	RuntimeGo      Runtime = "go"
	RuntimeFastAPI Runtime = "fastapi"

	DatabasePostgres Database = "postgres"
	DatabaseMySQL    Database = "mysql"
	DatabaseMongoDB  Database = "mongodb"
	DatabaseNone     Database = "none"

	CacheRedis Cache = "redis"
	CacheNone  Cache = "none"

	ProjectStatusCreating ProjectStatus = "creating"
	ProjectStatusReady    ProjectStatus = "ready"
	ProjectStatusFailed   ProjectStatus = "failed"
)

type Project struct {
	ID         string        `json:"id" db:"id"`
	Name       string        `json:"name" db:"name"`
	Slug       string        `json:"slug" db:"slug"`
	Runtime    Runtime       `json:"runtime" db:"runtime"`
	Database   Database      `json:"database" db:"database"`
	Cache      Cache         `json:"cache" db:"cache"`
	RepoURL    string        `json:"repo_url,omitempty" db:"repo_url"`
	ArgocdApp  string        `json:"argocd_app,omitempty" db:"argocd_app"`
	Status     ProjectStatus `json:"status" db:"status"`
	CreatedAt  time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at" db:"updated_at"`
}

type CreateProjectRequest struct {
	Name     string   `json:"name" binding:"required,min=2,max=50"`
	Runtime  Runtime  `json:"runtime" binding:"required,oneof=nextjs nestjs go fastapi"`
	Database Database `json:"database" binding:"required,oneof=postgres mysql mongodb none"`
	Cache    Cache    `json:"cache" binding:"required,oneof=redis none"`
}
