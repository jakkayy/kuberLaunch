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

func TestPromotionRepository(t *testing.T) {
	conn := testutil.Postgres(t)
	ctx := context.Background()
	promotions := repository.NewPromotionRepository(conn)

	alice := &model.User{GitHubID: 1, Login: "alice"}
	bob := &model.User{GitHubID: 2, Login: "bob"}
	users := repository.NewUserRepository(conn)
	for _, u := range []*model.User{alice, bob} {
		if err := users.Upsert(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	p := &model.Project{Name: "demo", Slug: "demo", Runtime: model.RuntimeGo,
		Database: model.DatabaseNone, Cache: model.CacheNone, Status: model.ProjectStatusReady, TemplateVersion: 3}
	if err := repository.NewProjectRepository(conn).Create(ctx, p); err != nil {
		t.Fatal(err)
	}

	req := &model.Promotion{ProjectID: p.ID, FromEnv: model.EnvStaging, ToEnv: model.EnvProd,
		ImageTag: "sha-abc1234", Status: model.PromotionPending, RequestedBy: alice.ID}
	if err := promotions.Create(ctx, req); err != nil {
		t.Fatal(err)
	}
	dup := *req
	if err := promotions.Create(ctx, &dup); !errors.Is(err, repository.ErrPromotionPending) {
		t.Fatalf("second pending promotion: err = %v", err)
	}

	ok, err := promotions.Decide(ctx, req.ID, bob.ID, model.PromotionApplied, "", "")
	if err != nil || !ok {
		t.Fatalf("Decide = %v, %v", ok, err)
	}
	if ok, _ := promotions.Decide(ctx, req.ID, bob.ID, model.PromotionRejected, "late", ""); ok {
		t.Fatal("a decided promotion must not be decided again")
	}

	got, err := promotions.Find(ctx, p.ID, req.ID)
	if err != nil || got.Status != model.PromotionApplied || got.RequestedByLogin != "alice" || got.DecidedByLogin != "bob" || got.DecidedAt == nil {
		t.Fatalf("Find = %+v, %v", got, err)
	}

	// Once decided, a new request for the same environment is allowed.
	again := &model.Promotion{ProjectID: p.ID, FromEnv: model.EnvStaging, ToEnv: model.EnvProd,
		ImageTag: "sha-def5678", Status: model.PromotionPending, RequestedBy: alice.ID}
	if err := promotions.Create(ctx, again); err != nil {
		t.Fatalf("new request after decision: %v", err)
	}

	applied := &model.Promotion{ProjectID: p.ID, FromEnv: model.EnvDev, ToEnv: model.EnvStaging,
		ImageTag: "sha-abc1234", Status: model.PromotionApplied, RequestedBy: bob.ID}
	if err := promotions.Record(ctx, applied); err != nil {
		t.Fatal(err)
	}
	list, err := promotions.ListByProject(ctx, p.ID)
	if err != nil || len(list) != 3 {
		t.Fatalf("ListByProject = %d, %v", len(list), err)
	}
}
