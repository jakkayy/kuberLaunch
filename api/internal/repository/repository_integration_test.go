//go:build integration

package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jakkayy/kuberlauncher/api/internal/model"
	"github.com/jakkayy/kuberlauncher/api/internal/repository"
	"github.com/jakkayy/kuberlauncher/api/internal/testutil"
)

func TestRepositories(t *testing.T) {
	conn := testutil.Postgres(t)
	ctx := context.Background()
	projects := repository.NewProjectRepository(conn)
	deployments := repository.NewDeploymentRepository(conn)

	p := &model.Project{
		Name: "Demo", Slug: "demo", Runtime: model.RuntimeGo,
		Database: model.DatabasePostgres, Cache: model.CacheNone, Status: model.ProjectStatusCreating,
	}

	t.Run("project create and find", func(t *testing.T) {
		if err := projects.Create(ctx, p); err != nil {
			t.Fatal(err)
		}
		got, err := projects.FindByID(ctx, p.ID)
		if err != nil || got == nil || got.Slug != "demo" {
			t.Fatalf("FindByID = %+v, %v", got, err)
		}
		exists, err := projects.SlugExists(ctx, "demo")
		if err != nil || !exists {
			t.Fatalf("SlugExists = %v, %v", exists, err)
		}
	})

	t.Run("project setters", func(t *testing.T) {
		if err := projects.SetRepoURL(ctx, p.ID, "https://github.com/acme/demo"); err != nil {
			t.Fatal(err)
		}
		if err := projects.SetArgoCDApp(ctx, p.ID, "demo"); err != nil {
			t.Fatal(err)
		}
		if err := projects.UpdateStatus(ctx, p.ID, model.ProjectStatusReady); err != nil {
			t.Fatal(err)
		}
		got, _ := projects.FindByID(ctx, p.ID)
		if got.RepoURL == "" || got.ArgocdApp != "demo" || got.Status != model.ProjectStatusReady {
			t.Fatalf("setters not persisted: %+v", got)
		}
	})

	t.Run("duplicate slug rejected", func(t *testing.T) {
		dup := *p
		if err := projects.Create(ctx, &dup); err == nil {
			t.Fatal("expected unique violation")
		}
	})

	t.Run("deployments lifecycle", func(t *testing.T) {
		d := &model.Deployment{ProjectID: p.ID, Branch: "main", TriggeredBy: "user", Status: model.DeploymentStatusBuilding}
		if err := deployments.Create(ctx, d); err != nil {
			t.Fatal(err)
		}
		active, err := deployments.FindActive(ctx)
		if err != nil || len(active) != 1 {
			t.Fatalf("FindActive = %v, %v", active, err)
		}
		second := &model.Deployment{ProjectID: p.ID, Branch: "main", TriggeredBy: "user", Status: model.DeploymentStatusPending}
		if err := deployments.Create(ctx, second); !errors.Is(err, repository.ErrDeploymentInProgress) {
			t.Fatalf("second active deployment: err = %v, want ErrDeploymentInProgress", err)
		}
		if err := deployments.SetWorkflowRunID(ctx, d.ID, 42); err != nil {
			t.Fatal(err)
		}
		if err := deployments.SetImage(ctx, d.ID, "ghcr.io/acme/demo:sha-1234567"); err != nil {
			t.Fatal(err)
		}
		if err := deployments.UpdateStatus(ctx, d.ID, model.DeploymentStatusSuccess); err != nil {
			t.Fatal(err)
		}
		got, _ := deployments.FindByID(ctx, d.ID)
		if *got.WorkflowRunID != 42 || got.Image == "" || got.Status != model.DeploymentStatusSuccess {
			t.Fatalf("deployment not updated: %+v", got)
		}
		active, _ = deployments.FindActive(ctx)
		if len(active) != 0 {
			t.Fatalf("terminal deployment still active: %v", active)
		}
	})

	t.Run("soft delete hides project but keeps slug taken", func(t *testing.T) {
		q := &model.Project{Name: "Gone", Slug: "gone", Runtime: model.RuntimeGo,
			Database: model.DatabaseNone, Cache: model.CacheNone, Status: model.ProjectStatusReady}
		if err := projects.Create(ctx, q); err != nil {
			t.Fatal(err)
		}
		if err := projects.SoftDelete(ctx, q.ID); err != nil {
			t.Fatal(err)
		}
		if got, _ := projects.FindByID(ctx, q.ID); got != nil {
			t.Fatalf("soft-deleted project still found: %+v", got)
		}
		all, _ := projects.FindAll(ctx)
		for _, p := range all {
			if p.ID == q.ID {
				t.Fatal("soft-deleted project listed")
			}
		}
		if exists, _ := projects.SlugExists(ctx, "gone"); !exists {
			t.Fatal("slug of soft-deleted project should stay taken")
		}
	})

	t.Run("delete cascades deployments", func(t *testing.T) {
		if err := projects.Delete(ctx, p.ID); err != nil {
			t.Fatal(err)
		}
		list, _ := deployments.FindByProject(ctx, p.ID)
		if len(list) != 0 {
			t.Fatalf("deployments not cascaded: %v", list)
		}
	})
}
