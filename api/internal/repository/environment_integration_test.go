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

func TestEnvironmentRepository(t *testing.T) {
	conn := testutil.Postgres(t)
	ctx := context.Background()
	envs := repository.NewEnvironmentRepository(conn)
	deployments := repository.NewDeploymentRepository(conn)

	p := &model.Project{Name: "demo", Slug: "demo", Runtime: model.RuntimeGo,
		Database: model.DatabaseNone, Cache: model.CacheNone, Status: model.ProjectStatusReady, TemplateVersion: 3}
	if err := repository.NewProjectRepository(conn).Create(ctx, p); err != nil {
		t.Fatal(err)
	}

	for _, env := range []model.EnvName{model.EnvProd, model.EnvDev, model.EnvStaging} {
		e := &model.Environment{ProjectID: p.ID, Name: env, Namespace: model.Namespace("demo", env), ArgocdApp: model.AppName("demo", env)}
		if err := envs.Upsert(ctx, e); err != nil {
			t.Fatal(err)
		}
	}

	list, err := envs.ListByProject(ctx, p.ID)
	if err != nil || len(list) != 3 || list[0].Name != model.EnvDev || list[2].Name != model.EnvProd {
		t.Fatalf("ListByProject = %+v, %v (want dev, staging, prod order)", list, err)
	}
	if e, _ := envs.FindByArgoApp(ctx, "demo-staging"); e == nil || e.Name != model.EnvStaging {
		t.Fatalf("FindByArgoApp = %+v", e)
	}
	if e, _ := envs.Find(ctx, p.ID, model.EnvProd); e == nil || e.Namespace != "demo-prod" {
		t.Fatalf("Find = %+v", e)
	}

	// One active deployment per environment, not per project.
	dev := &model.Deployment{ProjectID: p.ID, Environment: model.EnvDev, Branch: "main", TriggeredBy: "user", Status: model.DeploymentStatusBuilding}
	staging := &model.Deployment{ProjectID: p.ID, Environment: model.EnvStaging, Branch: "main", TriggeredBy: "promotion", Status: model.DeploymentStatusDeploying}
	if err := deployments.Create(ctx, dev); err != nil {
		t.Fatal(err)
	}
	if err := deployments.Create(ctx, staging); err != nil {
		t.Fatalf("staging deployment alongside dev: %v", err)
	}
	again := &model.Deployment{ProjectID: p.ID, Environment: model.EnvDev, Branch: "main", TriggeredBy: "user", Status: model.DeploymentStatusPending}
	if err := deployments.Create(ctx, again); !errors.Is(err, repository.ErrDeploymentInProgress) {
		t.Fatalf("second dev deployment: err = %v", err)
	}
	got, _ := deployments.FindByID(ctx, staging.ID)
	if got.Environment != model.EnvStaging {
		t.Fatalf("environment not stored: %+v", got)
	}
}
