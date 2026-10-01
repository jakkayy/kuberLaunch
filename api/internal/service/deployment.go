package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jakkayy/kuberlauncher/api/internal/argocd"
	gh "github.com/jakkayy/kuberlauncher/api/internal/github"
	"github.com/jakkayy/kuberlauncher/api/internal/model"
	"github.com/jakkayy/kuberlauncher/api/internal/repository"
)

type deploymentStore interface {
	Create(ctx context.Context, d *model.Deployment) error
	FindByID(ctx context.Context, id string) (*model.Deployment, error)
	FindByProject(ctx context.Context, projectID string) ([]model.Deployment, error)
	FindActive(ctx context.Context) ([]model.Deployment, error)
	UpdateStatus(ctx context.Context, id string, status model.DeploymentStatus) error
	SetWorkflowRunID(ctx context.Context, id string, runID int64) error
	SetImage(ctx context.Context, id, image string) error
}

type projectFinder interface {
	FindByID(ctx context.Context, id string) (*model.Project, error)
}

type workflowAPI interface {
	Owner() string
	TriggerWorkflow(ctx context.Context, slug, branch string) error
	FindDispatchedRun(ctx context.Context, slug, branch string, since time.Time) (*gh.WorkflowRun, error)
	GetWorkflowRun(ctx context.Context, slug string, runID int64) (*gh.WorkflowRun, error)
	GetFileContent(ctx context.Context, slug, path string) (string, error)
	UpdateFile(ctx context.Context, slug, path, content, message string) error
}

type appStatusAPI interface {
	GetAppStatus(ctx context.Context, appName string) (*argocd.AppStatus, error)
}

type DeploymentService struct {
	deployRepo  deploymentStore
	projectRepo projectFinder
	github      workflowAPI  // nil when GitHub is not configured
	argocd      appStatusAPI // nil when ArgoCD is not configured
}

func NewDeploymentService(
	deployRepo *repository.DeploymentRepository,
	projectRepo *repository.ProjectRepository,
	github *gh.Client,
	argocd *argocd.Client,
) *DeploymentService {
	s := &DeploymentService{deployRepo: deployRepo, projectRepo: projectRepo}
	// Assign only non-nil pointers so the interface fields stay nil when an
	// integration is disabled.
	if github != nil {
		s.github = github
	}
	if argocd != nil {
		s.argocd = argocd
	}
	return s
}

// DeployBranch is the only branch that CI promotes into helm/values.yaml,
// so it is the only branch whose deployment ArgoCD will actually roll out.
const DeployBranch = "main"

// Trigger creates a deployment record and dispatches a workflow_dispatch event on GitHub.
func (s *DeploymentService) Trigger(ctx context.Context, projectID, branch string) (*model.Deployment, error) {
	p, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil || p == nil {
		return nil, fmt.Errorf("project not found")
	}
	if p.RepoURL == "" {
		return nil, fmt.Errorf("project has no GitHub repo — connect to GitHub first")
	}
	if s.github == nil {
		return nil, fmt.Errorf("GitHub integration not configured")
	}
	if branch != DeployBranch {
		return nil, fmt.Errorf("%w: only the %s branch can be deployed; other branches only build images", ErrInvalid, DeployBranch)
	}

	d := &model.Deployment{
		ProjectID:   projectID,
		Branch:      branch,
		TriggeredBy: "user",
		Status:      model.DeploymentStatusPending,
	}
	if err := s.deployRepo.Create(ctx, d); err != nil {
		return nil, fmt.Errorf("create deployment: %w", err)
	}

	// Push updated CI workflow if it doesn't have workflow_dispatch
	if err := s.ensureWorkflowDispatch(ctx, p.Slug); err != nil {
		log.Printf("warn: could not update CI workflow for %s: %v", p.Slug, err)
	}

	// Trigger GitHub Actions
	if err := s.github.TriggerWorkflow(ctx, p.Slug, branch); err != nil {
		_ = s.deployRepo.UpdateStatus(ctx, d.ID, model.DeploymentStatusFailed)
		return nil, fmt.Errorf("trigger workflow: %w", err)
	}

	_ = s.deployRepo.UpdateStatus(ctx, d.ID, model.DeploymentStatusBuilding)
	d.Status = model.DeploymentStatusBuilding

	return d, nil
}

// GetByID returns a single deployment.
func (s *DeploymentService) GetByID(ctx context.Context, id string) (*model.Deployment, error) {
	return s.deployRepo.FindByID(ctx, id)
}

// ListByProject returns recent deployments for a project.
func (s *DeploymentService) ListByProject(ctx context.Context, projectID string) ([]model.Deployment, error) {
	return s.deployRepo.FindByProject(ctx, projectID)
}

// DeploymentTimeout is how long a deployment may stay non-terminal before it is marked failed.
const DeploymentTimeout = 30 * time.Minute

// Watch polls every active deployment on each tick until ctx is cancelled.
// It runs server-side so deployments progress even when no browser is watching.
func (s *DeploymentService) Watch(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pollActive(ctx)
		}
	}
}

func (s *DeploymentService) pollActive(ctx context.Context) {
	active, err := s.deployRepo.FindActive(ctx)
	if err != nil {
		log.Printf("watch deployments: %v", err)
		return
	}
	for _, d := range active {
		if time.Since(d.CreatedAt) > DeploymentTimeout {
			log.Printf("deployment %s timed out after %s", d.ID, DeploymentTimeout)
			_ = s.deployRepo.UpdateStatus(ctx, d.ID, model.DeploymentStatusFailed)
			continue
		}
		if _, err := s.PollStatus(ctx, d.ID); err != nil {
			log.Printf("poll deployment %s: %v", d.ID, err)
		}
	}
}

// PollStatus checks GitHub Actions + ArgoCD and updates the deployment status.
// Returns the updated status.
func (s *DeploymentService) PollStatus(ctx context.Context, deployID string) (model.DeploymentStatus, error) {
	d, err := s.deployRepo.FindByID(ctx, deployID)
	if err != nil || d == nil {
		return model.DeploymentStatusFailed, fmt.Errorf("deployment not found")
	}
	if d.Status.IsTerminal() {
		return d.Status, nil
	}
	p, err := s.projectRepo.FindByID(ctx, d.ProjectID)
	if err != nil || p == nil {
		return d.Status, fmt.Errorf("project not found")
	}

	// Phase 1: poll the GitHub Actions run that this deployment dispatched
	if d.Status == model.DeploymentStatusBuilding && s.github != nil {
		run, err := s.findRun(ctx, d, p.Slug)
		if err != nil || run == nil {
			return model.DeploymentStatusBuilding, err
		}
		if d.WorkflowRunID == nil {
			_ = s.deployRepo.SetWorkflowRunID(ctx, deployID, run.ID)
		}
		switch run.Conclusion {
		case "success":
			_ = s.deployRepo.SetImage(ctx, deployID, expectedImage(s.github.Owner(), p.Slug, run.HeadSHA))
			_ = s.deployRepo.UpdateStatus(ctx, deployID, model.DeploymentStatusDeploying)
			return model.DeploymentStatusDeploying, nil
		case "failure", "cancelled", "timed_out", "startup_failure", "action_required":
			_ = s.deployRepo.UpdateStatus(ctx, deployID, model.DeploymentStatusFailed)
			return model.DeploymentStatusFailed, nil
		}
		return model.DeploymentStatusBuilding, nil
	}

	// Phase 2: poll ArgoCD for sync + health
	if d.Status == model.DeploymentStatusDeploying && p.ArgocdApp != "" && s.argocd != nil {
		appStatus, err := s.argocd.GetAppStatus(ctx, p.ArgocdApp)
		if err == nil {
			if appStatus.Health == "Healthy" && appStatus.Sync == "Synced" && runsImage(appStatus.Images, d.Image) {
				_ = s.deployRepo.UpdateStatus(ctx, deployID, model.DeploymentStatusSuccess)
				return model.DeploymentStatusSuccess, nil
			}
			if appStatus.Health == "Degraded" {
				_ = s.deployRepo.UpdateStatus(ctx, deployID, model.DeploymentStatusFailed)
				return model.DeploymentStatusFailed, nil
			}
		}
	}

	return d.Status, nil
}

// expectedImage is the image tag the generated CI workflow pushes for a commit.
func expectedImage(owner, slug, sha string) string {
	if len(sha) > 7 {
		sha = sha[:7]
	}
	return strings.ToLower(fmt.Sprintf("ghcr.io/%s/%s:sha-%s", owner, slug, sha))
}

// runsImage reports whether ArgoCD shows the expected image running. A synced
// and healthy app that still runs the previous tag has not deployed this build.
// If ArgoCD reports no images (older versions), the check is skipped.
func runsImage(images []string, expected string) bool {
	if expected == "" || len(images) == 0 {
		return true
	}
	for _, img := range images {
		if strings.EqualFold(img, expected) {
			return true
		}
	}
	return false
}

// runLookupSkew tolerates clock drift between this server and GitHub when
// matching a dispatched run to the deployment that triggered it.
const runLookupSkew = 30 * time.Second

// findRun returns the workflow run belonging to d: by stored ID once known,
// otherwise the first workflow_dispatch run on d.Branch created after d.
func (s *DeploymentService) findRun(ctx context.Context, d *model.Deployment, slug string) (*gh.WorkflowRun, error) {
	if d.WorkflowRunID != nil {
		return s.github.GetWorkflowRun(ctx, slug, *d.WorkflowRunID)
	}
	return s.github.FindDispatchedRun(ctx, slug, d.Branch, d.CreatedAt.Add(-runLookupSkew))
}

// ensureWorkflowDispatch pushes an updated ci.yml with workflow_dispatch if it's missing.
func (s *DeploymentService) ensureWorkflowDispatch(ctx context.Context, slug string) error {
	// Read current ci.yml from the repo
	content, err := s.github.GetFileContent(ctx, slug, ".github/workflows/ci.yml")
	if err != nil {
		return err
	}
	if contains(content, "workflow_dispatch") {
		return nil // already has it
	}
	updated := addWorkflowDispatch(content)
	return s.github.UpdateFile(ctx, slug, ".github/workflows/ci.yml", updated,
		"ci: add workflow_dispatch trigger")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

func addWorkflowDispatch(content string) string {
	old := "  pull_request:\n    branches: [main]"
	new := "  pull_request:\n    branches: [main]\n  workflow_dispatch:"
	for i := 0; i <= len(content)-len(old); i++ {
		if content[i:i+len(old)] == old {
			return content[:i] + new + content[i+len(old):]
		}
	}
	return content
}
