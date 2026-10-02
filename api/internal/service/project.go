package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jakkayy/kuberlauncher/api/internal/argocd"
	"github.com/jakkayy/kuberlauncher/api/internal/generator"
	gh "github.com/jakkayy/kuberlauncher/api/internal/github"
	"github.com/jakkayy/kuberlauncher/api/internal/grafana"
	"github.com/jakkayy/kuberlauncher/api/internal/kube"
	"github.com/jakkayy/kuberlauncher/api/internal/model"
	"github.com/jakkayy/kuberlauncher/api/internal/repository"
	"github.com/jakkayy/kuberlauncher/api/internal/validate"
	"github.com/jakkayy/kuberlauncher/api/internal/vault"
)

type ProjectService struct {
	repo      *repository.ProjectRepository
	envs      *repository.EnvironmentRepository
	github    *gh.Client
	argocd    *argocd.Client
	grafana   *grafana.Client
	vault     *vault.Client
	gen       generator.Settings
	hooks     WebhookTarget
	argocdURL string
	kube      *kube.Client
	pull      *kube.RegistryCredentials
}

// ProjectDeps are the ProjectService dependencies. Integration clients are
// nil when not configured.
type ProjectDeps struct {
	Projects     *repository.ProjectRepository
	Environments *repository.EnvironmentRepository
	GitHub       *gh.Client
	ArgoCD       *argocd.Client
	Grafana      *grafana.Client
	Vault        *vault.Client
	Generator    generator.Settings
	Hooks        WebhookTarget
	// ArgoCDPublicURL is the ArgoCD UI address shown to users.
	ArgoCDPublicURL string
	// Kube creates namespaces and image pull secrets; PullCredentials is nil
	// when images are public and no pull secret is needed.
	Kube            *kube.Client
	PullCredentials *kube.RegistryCredentials
}

// WebhookTarget is where generated repos send workflow_run events.
// URL is empty when webhooks are disabled.
type WebhookTarget struct {
	URL    string
	Secret string
}

func NewProjectService(d ProjectDeps) *ProjectService {
	if d.GitHub != nil {
		d.Generator.Owner = d.GitHub.Owner()
	}
	return &ProjectService{
		repo: d.Projects, envs: d.Environments, github: d.GitHub, argocd: d.ArgoCD,
		grafana: d.Grafana, vault: d.Vault, gen: d.Generator, hooks: d.Hooks,
		argocdURL: strings.TrimRight(d.ArgoCDPublicURL, "/"),
		kube:      d.Kube, pull: d.PullCredentials,
	}
}

// namespaces lists the Kubernetes namespaces a project deploys to.
func namespaces(p *model.Project) []string {
	var out []string
	for _, env := range model.Environments {
		out = append(out, model.Namespace(p.Slug, env))
	}
	return out
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

	// Stop workloads first: deleting the apps cascades to their Kubernetes resources.
	if s.argocd != nil && p.ArgocdApp != "" {
		if err := s.deleteArgoCDApps(ctx, p); err != nil {
			return err
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
		// Re-register so ArgoCD picks up the new layout (e.g. per-environment apps).
		if p.ArgocdApp != "" {
			if _, err := s.RegisterArgoCD(ctx, id); err != nil {
				return nil, fmt.Errorf("re-register ArgoCD after upgrade: %w", err)
			}
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
	s.keepImageTags(ctx, p.Slug, files)
	pr, err = s.github.OpenPullRequest(ctx, p.Slug, branch,
		fmt.Sprintf("chore: upgrade kuberLaunch templates to v%d", generator.TemplateVersion),
		fmt.Sprintf("Generated by kuberLaunch: updates the scaffolded files from template v%d to v%d.\n\n"+
			"Review the diff and keep any local changes you rely on before merging. "+
			"Deployed image tags are preserved.", p.TemplateVersion, generator.TemplateVersion),
		files)
	if err != nil {
		return nil, err
	}
	up.URL, up.State = pr.URL, "open"
	return up, nil
}

var imageTagRe = regexp.MustCompile(`(?m)^(\s*tag:\s*).*$`)

// keepImageTags copies the deployed image tag of each environment from the
// repo into the regenerated values files, so an upgrade never rolls a service
// back to "latest". Repos from before environments (template v1-2) kept the
// tag in values.yaml, which becomes dev's tag.
func (s *ProjectService) keepImageTags(ctx context.Context, slug string, files []generator.GeneratedFile) {
	legacy, _ := s.github.GetFileContent(ctx, slug, "helm/values.yaml")
	for _, env := range model.Environments {
		path := generator.ValuesFile(env)
		current, err := s.github.GetFileContent(ctx, slug, path)
		if err != nil && env == model.EnvDev {
			current = legacy
		}
		for i := range files {
			if files[i].Path == path {
				files[i].Content = preserveImageTag(current, files[i].Content)
			}
		}
	}
}

// preserveImageTag replaces the tag line in generated with the one in current.
func preserveImageTag(current, generated string) string {
	tagLine := imageTagRe.FindString(current)
	if tagLine == "" {
		return generated
	}
	return imageTagRe.ReplaceAllLiteralString(generated, tagLine)
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
