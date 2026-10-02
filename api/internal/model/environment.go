package model

import "time"

// EnvName is a deployment environment. Every project has all three.
type EnvName string

const (
	EnvDev     EnvName = "dev"
	EnvStaging EnvName = "staging"
	EnvProd    EnvName = "prod"
)

// Environments lists environments in promotion order.
var Environments = []EnvName{EnvDev, EnvStaging, EnvProd}

func (e EnvName) Valid() bool {
	return e == EnvDev || e == EnvStaging || e == EnvProd
}

// Namespace is the Kubernetes namespace of a project's environment.
func Namespace(slug string, env EnvName) string { return slug + "-" + string(env) }

// AppName is the ArgoCD Application of a project's environment.
func AppName(slug string, env EnvName) string { return slug + "-" + string(env) }

// AutoSync reports whether ArgoCD syncs env automatically. Production only
// changes through an explicit, approved promotion.
func (e EnvName) AutoSync() bool { return e != EnvProd }

type Environment struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      EnvName   `json:"name"`
	Namespace string    `json:"namespace"`
	ArgocdApp string    `json:"argocd_app"`
	CreatedAt time.Time `json:"created_at"`
}

// EnvironmentStatus is an environment with its live ArgoCD state.
type EnvironmentStatus struct {
	Environment
	Health   string   `json:"health"`
	Sync     string   `json:"sync"`
	Revision string   `json:"revision,omitempty"`
	Images   []string `json:"images,omitempty"`
	URL      string   `json:"argocd_url,omitempty"`
}
