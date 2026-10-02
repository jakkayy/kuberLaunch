package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jakkayy/kuberlauncher/api/internal/repository"
)

// Setup step names, in execution order.
const (
	StepGitHub     = "github"
	StepArgoCD     = "argocd"
	StepMonitoring = "monitoring"
)

var setupSteps = []string{StepGitHub, StepArgoCD, StepMonitoring}

const (
	stepPending = "pending"
	stepRunning = "running"
	stepDone    = "done"
	stepFailed  = "failed"
)

// setupTimeout bounds one background setup run.
const setupTimeout = 10 * time.Minute

type SetupStep struct {
	Step    string `json:"step"`
	Status  string `json:"status"` // pending | running | done | failed
	Message string `json:"message"`
}

type SetupState struct {
	Steps []SetupStep `json:"steps"`
	// Running is true while any step is running.
	Running bool `json:"running"`
	// Complete is true once every required step is done (monitoring is optional).
	Complete bool `json:"complete"`
	// Failed is true when a required step failed; Start resumes from it.
	Failed bool `json:"failed"`
}

// Finished reports whether no run is in progress or about to make progress.
func (s *SetupState) Finished() bool {
	return !s.Running && (s.Complete || s.Failed)
}

type SetupService struct {
	projects *ProjectService
	repo     *repository.SetupRepository
}

func NewSetupService(projects *ProjectService, repo *repository.SetupRepository) *SetupService {
	return &SetupService{projects: projects, repo: repo}
}

// State returns the persisted progress of every step.
func (s *SetupService) State(ctx context.Context, projectID string) (*SetupState, error) {
	rows, err := s.repo.Steps(ctx, projectID)
	if err != nil {
		return nil, err
	}
	state := &SetupState{Complete: true}
	for _, name := range setupSteps {
		row, ok := rows[name]
		step := SetupStep{Step: name, Status: stepPending}
		if ok {
			step.Status, step.Message = row.Status, row.Message
		}
		// A run that crashed (e.g. the API restarted) leaves a step running forever.
		if step.Status == stepRunning && time.Since(row.UpdatedAt) > setupTimeout {
			step.Status, step.Message = stepFailed, "interrupted; run setup again to resume"
		}
		switch {
		case step.Status == stepRunning:
			state.Running = true
		case step.Status == stepFailed && name != StepMonitoring:
			state.Failed = true
		}
		if name != StepMonitoring && step.Status != stepDone {
			state.Complete = false
		}
		state.Steps = append(state.Steps, step)
	}
	return state, nil
}

// Start runs the remaining steps in the background and returns immediately.
// It is safe to call repeatedly: completed steps are skipped (each step is
// itself idempotent), and a run already in progress is not duplicated.
func (s *SetupService) Start(ctx context.Context, projectID string) error {
	if _, err := s.projects.GetByID(ctx, projectID); err != nil {
		return err
	}
	unlock, ok, err := s.repo.TryLock(ctx, projectID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: setup is already running", ErrConflict)
	}

	// Mark pending steps before returning so the first status read shows progress.
	rows, err := s.repo.Steps(ctx, projectID)
	if err != nil {
		unlock()
		return err
	}
	for _, name := range setupSteps {
		if rows[name].Status != stepDone {
			_ = s.repo.SetStep(ctx, projectID, name, stepPending, "")
		}
	}

	go func() {
		defer unlock()
		runCtx, cancel := context.WithTimeout(context.Background(), setupTimeout)
		defer cancel()
		s.run(runCtx, projectID)
	}()
	return nil
}

func (s *SetupService) run(ctx context.Context, projectID string) {
	type stepFunc func(context.Context, string) (string, error)
	funcs := map[string]stepFunc{
		StepGitHub:     s.projects.ConnectGitHub,
		StepArgoCD:     s.projects.RegisterArgoCD,
		StepMonitoring: s.projects.SetupMonitoring,
	}
	rows, _ := s.repo.Steps(ctx, projectID)

	for _, name := range setupSteps {
		if rows[name].Status == stepDone {
			continue
		}
		_ = s.repo.SetStep(ctx, projectID, name, stepRunning, "")
		msg, err := funcs[name](ctx, projectID)
		if err != nil {
			log.Printf("setup %s/%s: %v", projectID, name, err)
			_ = s.repo.SetStep(ctx, projectID, name, stepFailed, err.Error())
			if name == StepMonitoring {
				continue // optional: the service works without a dashboard
			}
			return
		}
		_ = s.repo.SetStep(ctx, projectID, name, stepDone, msg)
	}
}
