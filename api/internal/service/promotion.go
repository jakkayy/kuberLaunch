package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jakkayy/kuberlauncher/api/internal/argocd"
	"github.com/jakkayy/kuberlauncher/api/internal/generator"
	gh "github.com/jakkayy/kuberlauncher/api/internal/github"
	"github.com/jakkayy/kuberlauncher/api/internal/model"
	"github.com/jakkayy/kuberlauncher/api/internal/repository"
)

// PromotionService moves an already-built image from one environment to the
// next by committing its tag to the target's values file. Nothing is rebuilt,
// so what runs in prod is byte-for-byte what was tested in staging.
type PromotionService struct {
	projects    *repository.ProjectRepository
	envs        *repository.EnvironmentRepository
	promotions  *repository.PromotionRepository
	deployments *repository.DeploymentRepository
	github      *gh.Client
	argocd      *argocd.Client
}

func NewPromotionService(projects *repository.ProjectRepository, envs *repository.EnvironmentRepository,
	promotions *repository.PromotionRepository, deployments *repository.DeploymentRepository,
	github *gh.Client, argocd *argocd.Client) *PromotionService {
	return &PromotionService{projects: projects, envs: envs, promotions: promotions,
		deployments: deployments, github: github, argocd: argocd}
}

func (s *PromotionService) List(ctx context.Context, projectID string) ([]model.Promotion, error) {
	return s.promotions.ListByProject(ctx, projectID)
}

// Promote promotes the image deployed in the environment before to into to.
// Promotions into environments that require approval are only requested.
func (s *PromotionService) Promote(ctx context.Context, u *model.User, projectID string, to model.EnvName) (*model.Promotion, error) {
	from, ok := model.PromotionSource(to)
	if !ok {
		return nil, fmt.Errorf("%w: %q cannot be promoted into (use staging or prod)", ErrInvalid, to)
	}
	p, src, dst, err := s.load(ctx, projectID, from, to)
	if err != nil {
		return nil, err
	}

	tag, err := s.imageTag(ctx, p.Slug, from)
	if err != nil {
		return nil, err
	}
	if tag == "" || tag == "latest" {
		return nil, fmt.Errorf("%w: nothing has been deployed to %s yet", ErrInvalid, from)
	}
	if current, _ := s.imageTag(ctx, p.Slug, to); current == tag {
		return nil, fmt.Errorf("%w: %s already runs %s", ErrConflict, to, tag)
	}
	// Only promote something that is actually running well in the source.
	if s.argocd != nil {
		if st, err := s.argocd.GetAppStatus(ctx, src.ArgocdApp); err == nil && st.Health != "Healthy" {
			return nil, fmt.Errorf("%w: %s is %s; promote only a healthy environment", ErrInvalid, from, st.Health)
		}
	}

	pr := &model.Promotion{ProjectID: p.ID, FromEnv: from, ToEnv: to, ImageTag: tag, RequestedBy: u.ID, RequestedByLogin: u.Login}
	if to.RequiresApproval() {
		pr.Status = model.PromotionPending
		if err := s.promotions.Create(ctx, pr); err != nil {
			if errors.Is(err, repository.ErrPromotionPending) {
				return nil, fmt.Errorf("%w: %v", ErrConflict, err)
			}
			return nil, err
		}
		return pr, nil
	}

	d, err := s.apply(ctx, p, dst, tag)
	pr.Status, pr.Message = model.PromotionApplied, ""
	if d != nil {
		pr.DeploymentID = d.ID
	}
	if err != nil {
		pr.Status, pr.Message = model.PromotionFailed, err.Error()
	}
	if rerr := s.promotions.Record(ctx, pr); rerr != nil {
		return nil, rerr
	}
	return pr, err
}

// Approve applies a pending promotion. The requester cannot approve their own
// request (four-eyes principle).
func (s *PromotionService) Approve(ctx context.Context, u *model.User, projectID, promotionID string) (*model.Promotion, error) {
	pr, err := s.pending(ctx, projectID, promotionID)
	if err != nil {
		return nil, err
	}
	if pr.RequestedBy == u.ID {
		return nil, fmt.Errorf("%w: a different owner must approve your promotion request", ErrForbidden)
	}
	p, _, dst, err := s.load(ctx, projectID, pr.FromEnv, pr.ToEnv)
	if err != nil {
		return nil, err
	}
	// Claim the request first so two approvers cannot both apply it.
	claimed, err := s.promotions.Decide(ctx, pr.ID, u.ID, model.PromotionApplied, "", "")
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, fmt.Errorf("%w: promotion was already decided", ErrConflict)
	}

	d, applyErr := s.apply(ctx, p, dst, pr.ImageTag)
	status, msg, depID := model.PromotionApplied, "", ""
	if d != nil {
		depID = d.ID
	}
	if applyErr != nil {
		status, msg = model.PromotionFailed, applyErr.Error()
	}
	if err := s.promotions.SetResult(ctx, pr.ID, status, msg, depID); err != nil {
		return nil, err
	}
	pr, _ = s.promotions.Find(ctx, projectID, promotionID)
	return pr, applyErr
}

func (s *PromotionService) Reject(ctx context.Context, u *model.User, projectID, promotionID, reason string) (*model.Promotion, error) {
	pr, err := s.pending(ctx, projectID, promotionID)
	if err != nil {
		return nil, err
	}
	ok, err := s.promotions.Decide(ctx, pr.ID, u.ID, model.PromotionRejected, reason, "")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: promotion was already decided", ErrConflict)
	}
	return s.promotions.Find(ctx, projectID, promotionID)
}

func (s *PromotionService) pending(ctx context.Context, projectID, id string) (*model.Promotion, error) {
	pr, err := s.promotions.Find(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	if pr == nil {
		return nil, fmt.Errorf("%w: promotion", ErrNotFound)
	}
	if pr.Status != model.PromotionPending {
		return nil, fmt.Errorf("%w: promotion is %s", ErrConflict, pr.Status)
	}
	return pr, nil
}

func (s *PromotionService) load(ctx context.Context, projectID string, from, to model.EnvName) (*model.Project, *model.Environment, *model.Environment, error) {
	p, err := s.projects.FindByID(ctx, projectID)
	if err != nil {
		return nil, nil, nil, err
	}
	if p == nil {
		return nil, nil, nil, fmt.Errorf("project not found")
	}
	if s.github == nil || p.RepoURL == "" {
		return nil, nil, nil, fmt.Errorf("%w: project has no GitHub repo", ErrInvalid)
	}
	if p.TemplateVersion < envTemplateVersion {
		return nil, nil, nil, fmt.Errorf("%w: upgrade the project templates to use environments", ErrInvalid)
	}
	src, err := s.envs.Find(ctx, p.ID, from)
	if err != nil {
		return nil, nil, nil, err
	}
	dst, err := s.envs.Find(ctx, p.ID, to)
	if err != nil {
		return nil, nil, nil, err
	}
	if src == nil || dst == nil || src.ArgocdApp == "" || dst.ArgocdApp == "" {
		return nil, nil, nil, fmt.Errorf("%w: register the project in ArgoCD first", ErrInvalid)
	}
	return p, src, dst, nil
}

// apply commits tag into env's values file and records a deployment that the
// watcher completes once ArgoCD runs the image.
func (s *PromotionService) apply(ctx context.Context, p *model.Project, env *model.Environment, tag string) (*model.Deployment, error) {
	d := &model.Deployment{
		ProjectID:   p.ID,
		Environment: env.Name,
		Branch:      DeployBranch,
		TriggeredBy: "promotion",
		Status:      model.DeploymentStatusDeploying,
		Image:       strings.ToLower(fmt.Sprintf("ghcr.io/%s/%s:%s", s.github.Owner(), p.Slug, tag)),
	}
	if err := s.deployments.Create(ctx, d); err != nil {
		if errors.Is(err, repository.ErrDeploymentInProgress) {
			return nil, fmt.Errorf("%w: %v", ErrConflict, err)
		}
		return nil, err
	}
	fail := func(err error) (*model.Deployment, error) {
		_ = s.deployments.UpdateStatus(ctx, d.ID, model.DeploymentStatusFailed)
		d.Status = model.DeploymentStatusFailed
		return d, err
	}

	path := generator.ValuesFile(env.Name)
	content, err := s.github.GetFileContent(ctx, p.Slug, path)
	if err != nil {
		return fail(fmt.Errorf("read %s: %w", path, err))
	}
	updated, ok := setImageTag(content, tag)
	if !ok {
		return fail(fmt.Errorf("%s has no image tag line", path))
	}
	// [skip ci]: the image already exists; do not rebuild or bump dev.
	msg := fmt.Sprintf("chore(%s): promote %s [skip ci]", env.Name, tag)
	if err := s.github.UpdateFile(ctx, p.Slug, path, updated, msg); err != nil {
		return fail(fmt.Errorf("commit %s: %w", path, err))
	}

	// Environments without auto-sync (prod) are synced explicitly.
	if !env.Name.AutoSync() && s.argocd != nil {
		if err := s.argocd.SyncApp(ctx, env.ArgocdApp); err != nil {
			return fail(fmt.Errorf("sync %s: %w", env.ArgocdApp, err))
		}
	}
	return d, nil
}

func (s *PromotionService) imageTag(ctx context.Context, slug string, env model.EnvName) (string, error) {
	content, err := s.github.GetFileContent(ctx, slug, generator.ValuesFile(env))
	if err != nil {
		return "", fmt.Errorf("read %s values: %w", env, err)
	}
	return parseImageTag(content), nil
}

var tagValueRe = regexp.MustCompile(`(?m)^(\s*tag:\s*)"?([^"\s#]*)"?`)

func parseImageTag(values string) string {
	m := tagValueRe.FindStringSubmatch(values)
	if m == nil {
		return ""
	}
	return m[2]
}

// setImageTag rewrites the tag line, keeping its indentation.
func setImageTag(values, tag string) (string, bool) {
	if !tagValueRe.MatchString(values) {
		return values, false
	}
	return tagValueRe.ReplaceAllString(values, `${1}"`+tag+`"`), true
}
