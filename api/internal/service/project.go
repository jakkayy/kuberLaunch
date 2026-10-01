package service

import (
	"context"
	"fmt"
	"regexp"

	"github.com/jakkayy/kuberlauncher/api/internal/argocd"
	"github.com/jakkayy/kuberlauncher/api/internal/generator"
	gh "github.com/jakkayy/kuberlauncher/api/internal/github"
	"github.com/jakkayy/kuberlauncher/api/internal/grafana"
	"github.com/jakkayy/kuberlauncher/api/internal/model"
	"github.com/jakkayy/kuberlauncher/api/internal/repository"
	"github.com/jakkayy/kuberlauncher/api/internal/validate"
	"github.com/jakkayy/kuberlauncher/api/internal/vault"
)

type ProjectService struct {
	repo    *repository.ProjectRepository
	github  *gh.Client
	argocd  *argocd.Client
	grafana *grafana.Client
	vault   *vault.Client
	gen     generator.Settings
	hooks   WebhookTarget
}

// WebhookTarget is where generated repos send workflow_run events.
// URL is empty when webhooks are disabled.
type WebhookTarget struct {
	URL    string
	Secret string
}

func NewProjectService(repo *repository.ProjectRepository, github *gh.Client, argocd *argocd.Client,
	grafana *grafana.Client, vault *vault.Client, gen generator.Settings, hooks WebhookTarget) *ProjectService {
	if github != nil {
		gen.Owner = github.Owner()
	}
	return &ProjectService{repo: repo, github: github, argocd: argocd, grafana: grafana, vault: vault, gen: gen, hooks: hooks}
}

// namespaces lists the Kubernetes namespaces a project deploys to.
func namespaces(p *model.Project) []string {
	return []string{p.Slug + "-dev"}
}

func (s *ProjectService) Create(ctx context.Context, req model.CreateProjectRequest) (*model.Project, []generator.GeneratedFile, error) {
	slug := repository.ToSlug(req.Name)
	if err := validate.Slug(slug); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}

	exists, err := s.repo.SlugExists(ctx, slug)
	if err != nil {
		return nil, nil, err
	}
	if exists {
		return nil, nil, fmt.Errorf("%w: project with slug %q already exists", ErrConflict, slug)
	}

	p := &model.Project{
		Name:            req.Name,
		Slug:            slug,
		Runtime:         req.Runtime,
		Database:        req.Database,
		Cache:           req.Cache,
		Status:          model.ProjectStatusCreating,
		TemplateVersion: generator.TemplateVersion,
	}

	if err := s.repo.Create(ctx, p); err != nil {
		return nil, nil, fmt.Errorf("create project: %w", err)
	}

	files, err := generator.Generate(p, s.gen)
	if err != nil {
		_ = s.repo.UpdateStatus(ctx, p.ID, model.ProjectStatusFailed)
		return p, nil, fmt.Errorf("generate templates: %w", err)
	}

	if err := s.repo.UpdateStatus(ctx, p.ID, model.ProjectStatusReady); err != nil {
		return p, files, err
	}
	p.Status = model.ProjectStatusReady

	return p, files, nil
}

func (s *ProjectService) List(ctx context.Context) ([]model.Project, error) {
	return s.repo.FindAll(ctx)
}

func (s *ProjectService) GetByID(ctx context.Context, id string) (*model.Project, error) {
	p, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, fmt.Errorf("project not found")
	}
	return p, nil
}

// Delete tears down a project's running resources and hides it. confirm must
// equal the project slug. The GitHub repo is archived (not deleted) and the
// database row is soft-deleted, so code, history and audit trail are kept.
func (s *ProjectService) Delete(ctx context.Context, id, confirm string) error {
	p, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("project not found")
	}
	if confirm != p.Slug {
		return fmt.Errorf("%w: type the project slug %q to confirm deletion", ErrInvalid, p.Slug)
	}

	// Stop workloads first: deleting the app cascades to its Kubernetes resources.
	if s.argocd != nil && p.ArgocdApp != "" {
		if err := s.argocd.DeleteApp(ctx, p.ArgocdApp); err != nil {
			return fmt.Errorf("delete ArgoCD app: %w", err)
		}
		if err := s.argocd.DeleteProject(ctx, p.Slug); err != nil {
			return fmt.Errorf("delete ArgoCD project: %w", err)
		}
	}

	if s.vault != nil {
		if err := s.vault.DeleteProjectAccess(ctx, s.gen.VaultAuthMount, p.Slug); err != nil {
			return fmt.Errorf("delete Vault access: %w", err)
		}
	}

	if s.github != nil && p.RepoURL != "" {
		if err := s.github.ArchiveRepo(ctx, p.Slug); err != nil {
			return fmt.Errorf("archive GitHub repo: %w", err)
		}
	}

	return s.repo.SoftDelete(ctx, id)
}

func (s *ProjectService) Preview(ctx context.Context, id string) ([]generator.GeneratedFile, error) {
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return generator.Generate(p, s.gen)
}

func (s *ProjectService) SetupMonitoring(ctx context.Context, id string) (string, error) {
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	if p.GrafanaURL != "" {
		return p.GrafanaURL, nil
	}
	if s.grafana == nil {
		return "", fmt.Errorf("Grafana integration not configured (GRAFANA_PASSWORD missing)")
	}

	folderUID, err := s.grafana.EnsureFolder(ctx, p.Slug)
	if err != nil {
		return "", fmt.Errorf("create folder: %w", err)
	}

	dashURL, err := s.grafana.CreateDashboard(ctx, p.Slug, folderUID)
	if err != nil {
		return "", fmt.Errorf("create dashboard: %w", err)
	}

	if err := s.repo.SetGrafanaURL(ctx, id, dashURL); err != nil {
		return dashURL, err
	}
	return dashURL, nil
}

// RepairGitHub re-pushes all generated files to the existing GitHub repo.
// Use this to fix incorrectly encoded files.
func (s *ProjectService) RepairGitHub(ctx context.Context, id string) error {
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if p.RepoURL == "" {
		return fmt.Errorf("project has no GitHub repo")
	}
	if s.github == nil {
		return fmt.Errorf("GitHub integration not configured")
	}
	files, err := generator.Generate(p, s.gen)
	if err != nil {
		return fmt.Errorf("generate files: %w", err)
	}
	if err := s.github.PushFiles(ctx, p.Slug, files); err != nil {
		return err
	}
	return s.repo.SetTemplateVersion(ctx, id, generator.TemplateVersion)
}

// TemplateUpgrade reports the upgrade pull request for a project.
type TemplateUpgrade struct {
	From  int    `json:"from"`
	To    int    `json:"to"`
	URL   string `json:"url,omitempty"`
	State string `json:"state"` // up_to_date | open | merged
}

func upgradeBranch(version int) string {
	return fmt.Sprintf("kuberlaunch/templates-v%d", version)
}

// UpgradeTemplates opens a pull request that updates the repo's generated
// files to the current template version, so changes are reviewed instead of
// overwriting user edits. Calling it again reports the PR state and, once
// the PR is merged, records the new version.
func (s *ProjectService) UpgradeTemplates(ctx context.Context, id string) (*TemplateUpgrade, error) {
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	up := &TemplateUpgrade{From: p.TemplateVersion, To: generator.TemplateVersion}
	if p.TemplateVersion >= generator.TemplateVersion {
		up.State = "up_to_date"
		return up, nil
	}
	if p.RepoURL == "" {
		// Nothing pushed yet: the next ConnectGitHub uses current templates.
		if err := s.repo.SetTemplateVersion(ctx, id, generator.TemplateVersion); err != nil {
			return nil, err
		}
		up.State = "up_to_date"
		return up, nil
	}
	if s.github == nil {
		return nil, fmt.Errorf("GitHub integration not configured")
	}

	branch := upgradeBranch(generator.TemplateVersion)
	pr, err := s.github.FindPullRequest(ctx, p.Slug, branch)
	if err != nil {
		return nil, fmt.Errorf("find upgrade PR: %w", err)
	}
	if pr != nil && pr.Merged {
		if err := s.repo.SetTemplateVersion(ctx, id, generator.TemplateVersion); err != nil {
			return nil, err
		}
		up.URL, up.State = pr.URL, "merged"
		return up, nil
	}
	if pr != nil && pr.State == "open" {
		up.URL, up.State = pr.URL, "open"
		return up, nil
	}

	files, err := generator.Generate(p, s.gen)
	if err != nil {
		return nil, fmt.Errorf("generate files: %w", err)
	}
	if err := s.keepImageTag(ctx, p.Slug, files); err != nil {
		return nil, err
	}
	pr, err = s.github.OpenPullRequest(ctx, p.Slug, branch,
		fmt.Sprintf("chore: upgrade kuberLaunch templates to v%d", generator.TemplateVersion),
		fmt.Sprintf("Generated by kuberLaunch: updates the scaffolded files from template v%d to v%d.\n\n"+
			"Review the diff and keep any local changes you rely on before merging. "+
			"The deployed image tag in helm/values.yaml is preserved.", p.TemplateVersion, generator.TemplateVersion),
		files)
	if err != nil {
		return nil, err
	}
	up.URL, up.State = pr.URL, "open"
	return up, nil
}

var imageTagRe = regexp.MustCompile(`(?m)^(\s*tag:\s*).*$`)

// keepImageTag copies the currently deployed image tag from the repo's
// helm/values.yaml into the regenerated one, so an upgrade does not roll the
// service back to "latest".
func (s *ProjectService) keepImageTag(ctx context.Context, slug string, files []generator.GeneratedFile) error {
	current, err := s.github.GetFileContent(ctx, slug, "helm/values.yaml")
	if err != nil {
		return nil // file missing upstream: nothing to preserve
	}
	for i := range files {
		if files[i].Path == "helm/values.yaml" {
			files[i].Content = preserveImageTag(current, files[i].Content)
		}
	}
	return nil
}

// preserveImageTag replaces the tag line in generated with the one in current.
func preserveImageTag(current, generated string) string {
	tagLine := imageTagRe.FindString(current)
	if tagLine == "" {
		return generated
	}
	return imageTagRe.ReplaceAllLiteralString(generated, tagLine)
}

func (s *ProjectService) RegisterArgoCD(ctx context.Context, id string) (string, error) {
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return "", err
	}

	if p.RepoURL == "" {
		return "", fmt.Errorf("project has no GitHub repo — connect to GitHub first")
	}

	if p.ArgocdApp != "" {
		return p.ArgocdApp, nil
	}

	if s.argocd == nil {
		return "", fmt.Errorf("ArgoCD integration not configured (ARGOCD_TOKEN missing)")
	}

	// Let the project's service account read its Vault secrets before the
	// first sync creates the SecretStore.
	if s.vault != nil {
		if err := s.vault.EnsureProjectAccess(ctx, s.gen.VaultAuthMount, p.Slug, namespaces(p)); err != nil {
			return "", fmt.Errorf("grant Vault access: %w", err)
		}
	}

	appName, err := s.argocd.RegisterApp(ctx, p.Slug, p.RepoURL)
	if err != nil {
		return "", fmt.Errorf("register ArgoCD app: %w", err)
	}

	if err := s.repo.SetArgoCDApp(ctx, id, appName); err != nil {
		return appName, err
	}

	return appName, nil
}

func (s *ProjectService) GetArgoCDStatus(ctx context.Context, id string) (map[string]string, error) {
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.ArgocdApp == "" {
		return map[string]string{"health": "", "sync": ""}, nil
	}
	if s.argocd == nil {
		return map[string]string{"health": "Unknown", "sync": "Unknown"}, nil
	}
	status, err := s.argocd.GetAppStatus(ctx, p.ArgocdApp)
	if err != nil {
		// ArgoCD ไม่ตอบหรือ login fail — คืน Unknown แทน error
		return map[string]string{"health": "Unknown", "sync": "Unknown"}, nil
	}
	return map[string]string{"health": status.Health, "sync": status.Sync}, nil
}

func (s *ProjectService) RollbackArgoCD(ctx context.Context, id string) error {
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if p.ArgocdApp == "" {
		return fmt.Errorf("project has no ArgoCD app")
	}
	if s.argocd == nil {
		return fmt.Errorf("ArgoCD integration not configured")
	}
	return s.argocd.RollbackApp(ctx, p.ArgocdApp)
}

func (s *ProjectService) ConnectGitHub(ctx context.Context, id string) (string, error) {
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return "", err
	}

	if p.RepoURL != "" {
		return p.RepoURL, nil
	}

	if s.github == nil {
		return "", fmt.Errorf("GitHub integration not configured (GITHUB_TOKEN missing)")
	}

	files, err := generator.Generate(p, s.gen)
	if err != nil {
		return "", fmt.Errorf("generate files: %w", err)
	}

	var repoURL string
	if s.github.RepoExists(ctx, p.Slug) {
		// Repo มีอยู่แล้ว (สร้างก่อนหน้านี้) — link และ re-push ไฟล์
		repoURL = fmt.Sprintf("https://github.com/%s/%s", s.github.Owner(), p.Slug)
		if err := s.github.PushFiles(ctx, p.Slug, files); err != nil {
			return "", fmt.Errorf("re-push files: %w", err)
		}
	} else {
		repoURL, err = s.github.CreateRepo(ctx, p.Slug, files)
		if err != nil {
			return "", fmt.Errorf("create GitHub repo: %w", err)
		}
	}

	if s.hooks.URL != "" {
		if err := s.github.EnsureWebhook(ctx, p.Slug, s.hooks.URL, s.hooks.Secret); err != nil {
			return "", fmt.Errorf("register webhook: %w", err)
		}
	}

	if err := s.repo.SetRepoURL(ctx, id, repoURL); err != nil {
		return repoURL, err
	}
	if err := s.repo.SetTemplateVersion(ctx, id, generator.TemplateVersion); err != nil {
		return repoURL, err
	}

	return repoURL, nil
}
