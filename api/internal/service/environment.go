package service

import (
	"context"
	"fmt"

	"github.com/jakkayy/kuberlauncher/api/internal/argocd"
	"github.com/jakkayy/kuberlauncher/api/internal/model"
)

// envTemplateVersion is the first template version with per-environment
// values files; older repos only have helm/values.yaml.
const envTemplateVersion = 3

// RegisterArgoCD registers one ArgoCD Application per environment inside the
// project's AppProject. It is idempotent and returns the dev app name.
func (s *ProjectService) RegisterArgoCD(ctx context.Context, id string) (string, error) {
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	if p.RepoURL == "" {
		return "", fmt.Errorf("project has no GitHub repo — connect to GitHub first")
	}
	if s.argocd == nil {
		return "", fmt.Errorf("ArgoCD integration not configured (ARGOCD_TOKEN missing)")
	}
	if p.TemplateVersion < envTemplateVersion {
		if p.ArgocdApp != "" {
			return p.ArgocdApp, nil // legacy single app keeps working until templates are upgraded
		}
		return "", fmt.Errorf("%w: upgrade the project templates before registering in ArgoCD", ErrInvalid)
	}

	// Let the project's service account read its Vault secrets before the
	// first sync creates the SecretStore.
	if s.vault != nil {
		if err := s.vault.EnsureProjectAccess(ctx, s.gen.VaultAuthMount, p.Slug, namespaces(p)); err != nil {
			return "", fmt.Errorf("grant Vault access: %w", err)
		}
	}
	if err := s.argocd.EnsureProject(ctx, p.Slug, p.RepoURL); err != nil {
		return "", err
	}

	devApp := model.AppName(p.Slug, model.EnvDev)
	if p.ArgocdApp != "" && p.ArgocdApp != devApp {
		// Pre-environment app named <slug>: hand its running resources over to
		// <slug>-dev (same release name and namespace) without downtime.
		if err := s.argocd.OrphanApp(ctx, p.ArgocdApp); err != nil {
			return "", fmt.Errorf("retire legacy ArgoCD app: %w", err)
		}
	}

	for _, env := range model.Environments {
		app := model.AppName(p.Slug, env)
		err := s.argocd.RegisterApp(ctx, argocd.AppSpec{
			Name:        app,
			Project:     p.Slug,
			RepoURL:     p.RepoURL,
			ReleaseName: p.Slug,
			ValueFiles:  []string{"values.yaml", "values-" + string(env) + ".yaml"},
			Namespace:   model.Namespace(p.Slug, env),
			AutoSync:    env.AutoSync(),
		})
		if err != nil {
			return "", fmt.Errorf("register %s: %w", app, err)
		}
		if err := s.envs.Upsert(ctx, &model.Environment{
			ProjectID: p.ID, Name: env, Namespace: model.Namespace(p.Slug, env), ArgocdApp: app,
		}); err != nil {
			return "", err
		}
	}

	if err := s.repo.SetArgoCDApp(ctx, id, devApp); err != nil {
		return devApp, err
	}
	return devApp, nil
}

func (s *ProjectService) deleteArgoCDApps(ctx context.Context, p *model.Project) error {
	envs, err := s.envs.ListByProject(ctx, p.ID)
	if err != nil {
		return err
	}
	apps := map[string]bool{p.ArgocdApp: true}
	for _, e := range envs {
		if e.ArgocdApp != "" {
			apps[e.ArgocdApp] = true
		}
	}
	for app := range apps {
		if err := s.argocd.DeleteApp(ctx, app); err != nil {
			return fmt.Errorf("delete ArgoCD app %s: %w", app, err)
		}
	}
	if err := s.argocd.DeleteProject(ctx, p.Slug); err != nil {
		return fmt.Errorf("delete ArgoCD project: %w", err)
	}
	return nil
}

// Environments returns every environment of a project with its live ArgoCD
// state. Unreachable ArgoCD yields "Unknown" rather than an error.
func (s *ProjectService) Environments(ctx context.Context, id string) ([]model.EnvironmentStatus, error) {
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	envs, err := s.envs.ListByProject(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	out := make([]model.EnvironmentStatus, 0, len(envs))
	for _, e := range envs {
		st := model.EnvironmentStatus{Environment: e, Health: "Unknown", Sync: "Unknown"}
		if s.argocdURL != "" && e.ArgocdApp != "" {
			st.URL = s.argocdURL + "/applications/argocd/" + e.ArgocdApp
		}
		if s.argocd != nil && e.ArgocdApp != "" {
			if live, err := s.argocd.GetAppStatus(ctx, e.ArgocdApp); err == nil {
				st.Health, st.Sync, st.Revision, st.Images = live.Health, live.Sync, live.Revision, live.Images
			}
		}
		out = append(out, st)
	}
	return out, nil
}

func (s *ProjectService) environment(ctx context.Context, p *model.Project, env model.EnvName) (*model.Environment, error) {
	if !env.Valid() {
		return nil, fmt.Errorf("%w: unknown environment %q", ErrInvalid, env)
	}
	e, err := s.envs.Find(ctx, p.ID, env)
	if err != nil {
		return nil, err
	}
	if e == nil || e.ArgocdApp == "" {
		return nil, fmt.Errorf("%w: environment %s is not registered in ArgoCD", ErrNotFound, env)
	}
	return e, nil
}

// Namespace returns the namespace of a project's environment.
func (s *ProjectService) Namespace(ctx context.Context, id string, env model.EnvName) (string, error) {
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	if !env.Valid() {
		return "", fmt.Errorf("%w: unknown environment %q", ErrInvalid, env)
	}
	return model.Namespace(p.Slug, env), nil
}

// RollbackArgoCD rolls an environment back to its previous synced revision.
func (s *ProjectService) RollbackArgoCD(ctx context.Context, id string, env model.EnvName) error {
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if s.argocd == nil {
		return fmt.Errorf("ArgoCD integration not configured")
	}
	e, err := s.environment(ctx, p, env)
	if err != nil {
		return err
	}
	return s.argocd.RollbackApp(ctx, e.ArgocdApp)
}
