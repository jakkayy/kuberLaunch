package service

import (
	"context"
	"fmt"

	"github.com/jakkayy/kuberlauncher/api/internal/argocd"
	"github.com/jakkayy/kuberlauncher/api/internal/generator"
	gh "github.com/jakkayy/kuberlauncher/api/internal/github"
	"github.com/jakkayy/kuberlauncher/api/internal/grafana"
	"github.com/jakkayy/kuberlauncher/api/internal/model"
	"github.com/jakkayy/kuberlauncher/api/internal/repository"
)

type ProjectService struct {
	repo    *repository.ProjectRepository
	github  *gh.Client
	argocd  *argocd.Client
	grafana *grafana.Client
}

func NewProjectService(repo *repository.ProjectRepository, github *gh.Client, argocd *argocd.Client, grafana *grafana.Client) *ProjectService {
	return &ProjectService{repo: repo, github: github, argocd: argocd, grafana: grafana}
}

func (s *ProjectService) githubOwner() string {
	if s.github != nil {
		return s.github.Owner()
	}
	return ""
}

func (s *ProjectService) Create(ctx context.Context, req model.CreateProjectRequest) (*model.Project, []generator.GeneratedFile, error) {
	slug := repository.ToSlug(req.Name)

	exists, err := s.repo.SlugExists(ctx, slug)
	if err != nil {
		return nil, nil, err
	}
	if exists {
		return nil, nil, fmt.Errorf("project with name %q already exists", req.Name)
	}

	p := &model.Project{
		Name:     req.Name,
		Slug:     slug,
		Runtime:  req.Runtime,
		Database: req.Database,
		Cache:    req.Cache,
		Status:   model.ProjectStatusCreating,
	}

	if err := s.repo.Create(ctx, p); err != nil {
		return nil, nil, fmt.Errorf("create project: %w", err)
	}

	files, err := generator.Generate(p, s.githubOwner())
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

func (s *ProjectService) Delete(ctx context.Context, id string) error {
	p, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("project not found")
	}

	if s.argocd != nil && p.ArgocdApp != "" {
		if err := s.argocd.DeleteApp(ctx, p.ArgocdApp); err != nil {
			return fmt.Errorf("delete ArgoCD app: %w", err)
		}
	}

	if s.github != nil && p.RepoURL != "" {
		if err := s.github.DeleteRepo(ctx, p.Slug); err != nil {
			return fmt.Errorf("delete GitHub repo: %w", err)
		}
	}

	return s.repo.Delete(ctx, id)
}

func (s *ProjectService) Preview(ctx context.Context, id string) ([]generator.GeneratedFile, error) {
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return generator.Generate(p, s.githubOwner())
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

	fullURL := "http://grafana.localhost:8090" + dashURL
	if err := s.repo.SetGrafanaURL(ctx, id, fullURL); err != nil {
		return fullURL, err
	}
	return fullURL, nil
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
	files, err := generator.Generate(p, s.githubOwner())
	if err != nil {
		return fmt.Errorf("generate files: %w", err)
	}
	return s.github.PushFiles(ctx, p.Slug, files)
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
		return "", fmt.Errorf("ArgoCD integration not configured (ARGOCD_PASSWORD missing)")
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

	files, err := generator.Generate(p, s.githubOwner())
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

	if err := s.repo.SetRepoURL(ctx, id, repoURL); err != nil {
		return repoURL, err
	}

	return repoURL, nil
}
