package service

import (
	"context"
	"fmt"

	"github.com/jakkayy/kuberlauncher/api/internal/generator"
	"github.com/jakkayy/kuberlauncher/api/internal/model"
	"github.com/jakkayy/kuberlauncher/api/internal/repository"
)

type ProjectService struct {
	repo *repository.ProjectRepository
}

func NewProjectService(repo *repository.ProjectRepository) *ProjectService {
	return &ProjectService{repo: repo}
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

	files, err := generator.Generate(p)
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
	return s.repo.Delete(ctx, id)
}

func (s *ProjectService) Preview(ctx context.Context, id string) ([]generator.GeneratedFile, error) {
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return generator.Generate(p)
}
