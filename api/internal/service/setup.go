package service

import (
	"context"
	"fmt"
)

type SetupStep struct {
	Name    string
	Status  string // pending | running | done | failed
	Message string
}

type SetupProgress struct {
	Step    string `json:"step"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Done    bool   `json:"done"`
}

// OneClickSetup runs ConnectGitHub → RegisterArgoCD → SetupMonitoring in sequence.
// Progress is reported via the channel. Caller must drain the channel.
func (s *ProjectService) OneClickSetup(ctx context.Context, id string) <-chan SetupProgress {
	ch := make(chan SetupProgress, 10)

	go func() {
		defer close(ch)

		emit := func(step, status, msg string, done bool) {
			ch <- SetupProgress{Step: step, Status: status, Message: msg, Done: done}
		}

		// Step 1: Connect GitHub
		emit("github", "running", "Creating GitHub repo…", false)
		repoURL, err := s.ConnectGitHub(ctx, id)
		if err != nil {
			emit("github", "failed", err.Error(), true)
			return
		}
		emit("github", "done", repoURL, false)

		// Step 2: Register ArgoCD
		emit("argocd", "running", "Registering in ArgoCD…", false)
		appName, err := s.RegisterArgoCD(ctx, id)
		if err != nil {
			emit("argocd", "failed", err.Error(), true)
			return
		}
		emit("argocd", "done", appName, false)

		// Step 3: Setup Monitoring
		emit("monitoring", "running", "Creating Grafana dashboard…", false)
		dashURL, err := s.SetupMonitoring(ctx, id)
		if err != nil {
			emit("monitoring", "failed", fmt.Sprintf("warning: %s", err.Error()), false)
		} else {
			emit("monitoring", "done", dashURL, false)
		}

		emit("complete", "done", "All done!", true)
	}()

	return ch
}
