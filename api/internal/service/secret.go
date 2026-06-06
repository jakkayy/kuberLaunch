package service

import (
	"context"
	"fmt"

	"github.com/jakkayy/kuberlauncher/api/internal/repository"
	"github.com/jakkayy/kuberlauncher/api/internal/vault"
)

type SecretService struct {
	projectRepo *repository.ProjectRepository
	vault       *vault.Client
}

func NewSecretService(projectRepo *repository.ProjectRepository, vault *vault.Client) *SecretService {
	return &SecretService{projectRepo: projectRepo, vault: vault}
}

func (s *SecretService) Set(ctx context.Context, projectID, key, value string) error {
	p, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil || p == nil {
		return fmt.Errorf("project not found")
	}
	if s.vault == nil {
		return fmt.Errorf("Vault integration not configured")
	}
	return s.vault.SetSecret(ctx, p.Slug, key, value)
}

func (s *SecretService) ListKeys(ctx context.Context, projectID string) ([]string, error) {
	p, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil || p == nil {
		return nil, fmt.Errorf("project not found")
	}
	if s.vault == nil {
		return []string{}, nil
	}
	return s.vault.ListSecretKeys(ctx, p.Slug)
}

func (s *SecretService) Delete(ctx context.Context, projectID, key string) error {
	p, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil || p == nil {
		return fmt.Errorf("project not found")
	}
	if s.vault == nil {
		return fmt.Errorf("Vault integration not configured")
	}
	return s.vault.DeleteSecret(ctx, p.Slug, key)
}
