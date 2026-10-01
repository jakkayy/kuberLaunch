package service

import (
	"context"
	"errors"
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
	FindActiveByProject(ctx context.Context, projectID string) ([]model.Deployment, error)
	UpdateStatus(ctx context.Context, id string, status model.DeploymentStatus) error
	SetWorkflowRunID(ctx context.Context, id string, runID int64) error
	SetImage(ctx context.Context, id, image string) error
}

type projectFinder interface {
	FindByID(ctx context.Context, id string) (*model.Project, error)
	FindBySlug(ctx context.Context, slug string) (*model.Project, error)
	FindByArgoApp(ctx context.Context, app string) (*model.Project, error)
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
		if errors.Is(err, repository.ErrDeploymentInProgress) {
			return nil, fmt.Errorf("%w: %v", ErrConflict, err)
		}
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
	if err != nil {
		return d.Status, err
	}
	if p == nil {
		// Project was deleted mid-deployment; nothing left to roll out.
		_ = s.deployRepo.UpdateStatus(ctx, deployID, model.DeploymentStatusFailed)
		return model.DeploymentStatusFailed, nil
	}

	// Phase 1: poll the GitHub Actions run that this deployment dispatched
	if d.Status == model.DeploymentStatusBuilding && s.github != nil {
		run, err := s.findRun(ctx, d, p.Slug)
		if err != nil || run == nil {
			return model.DeploymentStatusBuilding, err
		}
		return s.applyRun(ctx, d, p, run), nil
	}

	// Phase 2: poll ArgoCD for sync + health
	if d.Status == model.DeploymentStatusDeploying && p.ArgocdApp != "" && s.argocd != nil {
		appStatus, err := s.argocd.GetAppStatus(ctx, p.ArgocdApp)
		if err == nil {
			return s.applyAppStatus(ctx, d, appStatus), nil
		}
	}

	return d.Status, nil
}

// applyRun advances a building deployment from its workflow run.
func (s *DeploymentService) applyRun(ctx context.Context, d *model.Deployment, p *model.Project, run *gh.WorkflowRun) model.DeploymentStatus {
	if d.WorkflowRunID == nil {
		_ = s.deployRepo.SetWorkflowRunID(ctx, d.ID, run.ID)
		d.WorkflowRunID = &run.ID
	}
	switch run.Conclusion {
	case "success":
		d.Image = expectedImage(s.github.Owner(), p.Slug, run.HeadSHA)
		_ = s.deployRepo.SetImage(ctx, d.ID, d.Image)
		return s.setStatus(ctx, d, model.DeploymentStatusDeploying)
	case "failure", "cancelled", "timed_out", "startup_failure", "action_required":
		return s.setStatus(ctx, d, model.DeploymentStatusFailed)
	}
	return d.Status
}

// applyAppStatus advances a deploying deployment from the ArgoCD app state.
func (s *DeploymentService) applyAppStatus(ctx context.Context, d *model.Deployment, st *argocd.AppStatus) model.DeploymentStatus {
	switch {
	case st.Health == "Healthy" && st.Sync == "Synced" && runsImage(st.Images, d.Image):
		return s.setStatus(ctx, d, model.DeploymentStatusSuccess)
	case st.Health == "Degraded":
		return s.setStatus(ctx, d, model.DeploymentStatusFailed)
	}
	return d.Status
}

func (s *DeploymentService) setStatus(ctx context.Context, d *model.Deployment, status model.DeploymentStatus) model.DeploymentStatus {
	_ = s.deployRepo.UpdateStatus(ctx, d.ID, status)
	d.Status = status
	return status
}

// WorkflowRunEvent is the part of a GitHub workflow_run webhook we use.
type WorkflowRunEvent struct {
	Repo      string // repository name = project slug
	Branch    string
	Event     string // what triggered the run, e.g. workflow_dispatch
	CreatedAt time.Time
	Run       gh.WorkflowRun
}

// HandleWorkflowRun applies a workflow_run webhook to the deployment that
// dispatched the run, if any.
func (s *DeploymentService) HandleWorkflowRun(ctx context.Context, ev WorkflowRunEvent) error {
	if s.github == nil {
		return nil
	}
	p, err := s.projectRepo.FindBySlug(ctx, ev.Repo)
	if err != nil || p == nil {
		return err
	}
	active, err := s.deployRepo.FindActiveByProject(ctx, p.ID)
	if err != nil {
		return err
	}
	for i := range active {
		d := &active[i]
		if d.Status != model.DeploymentStatusBuilding || !runMatches(d, ev) {
			continue
		}
		s.applyRun(ctx, d, p, &ev.Run)
		return nil
	}
	return nil
}

// runMatches mirrors findRun: the stored run ID, or the first dispatched run
// on the deployment's branch created after the deployment.
func runMatches(d *model.Deployment, ev WorkflowRunEvent) bool {
	if d.WorkflowRunID != nil {
		return *d.WorkflowRunID == ev.Run.ID
	}
	return ev.Event == "workflow_dispatch" && ev.Branch == d.Branch &&
		!ev.CreatedAt.Before(d.CreatedAt.Add(-runLookupSkew))
}

// HandleAppStatus applies an ArgoCD notification for app to its project's
// deploying deployments.
func (s *DeploymentService) HandleAppStatus(ctx context.Context, app string, st *argocd.AppStatus) error {
	p, err := s.projectRepo.FindByArgoApp(ctx, app)
	if err != nil || p == nil {
		return err
	}
	active, err := s.deployRepo.FindActiveByProject(ctx, p.ID)
	if err != nil {
		return err
	}
	for i := range active {
		if active[i].Status == model.DeploymentStatusDeploying {
			s.applyAppStatus(ctx, &active[i], st)
		}
	}
	return nil
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
