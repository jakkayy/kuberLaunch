//go:build integration

package repository_test

import (
	"context"
	"testing"

	"github.com/jakkayy/kuberlauncher/api/internal/model"
	"github.com/jakkayy/kuberlauncher/api/internal/repository"
	"github.com/jakkayy/kuberlauncher/api/internal/testutil"
)

func TestSetupRepository(t *testing.T) {
	conn := testutil.Postgres(t)
	ctx := context.Background()
	repo := repository.NewSetupRepository(conn)

	p := &model.Project{Name: "demo", Slug: "demo", Runtime: model.RuntimeGo,
		Database: model.DatabaseNone, Cache: model.CacheNone, Status: model.ProjectStatusReady}
	if err := repository.NewProjectRepository(conn).Create(ctx, p); err != nil {
		t.Fatal(err)
	}

	t.Run("steps upsert", func(t *testing.T) {
		if err := repo.SetStep(ctx, p.ID, "github", "running", ""); err != nil {
			t.Fatal(err)
		}
		if err := repo.SetStep(ctx, p.ID, "github", "done", "https://github.com/acme/demo"); err != nil {
			t.Fatal(err)
		}
		steps, err := repo.Steps(ctx, p.ID)
		if err != nil || steps["github"].Status != "done" || steps["github"].Message == "" {
			t.Fatalf("steps = %+v, %v", steps, err)
		}
	})

	t.Run("advisory lock is exclusive", func(t *testing.T) {
		unlock, ok, err := repo.TryLock(ctx, p.ID)
		if err != nil || !ok {
			t.Fatalf("first lock: ok=%v err=%v", ok, err)
		}
		if _, ok, err := repo.TryLock(ctx, p.ID); err != nil || ok {
			t.Fatalf("second lock should fail: ok=%v err=%v", ok, err)
		}
		other, ok, _ := repo.TryLock(ctx, "other-project")
		if !ok {
			t.Fatal("lock for another project should succeed")
		}
		other()
		unlock()
		unlock2, ok, err := repo.TryLock(ctx, p.ID)
		if err != nil || !ok {
			t.Fatalf("lock after unlock: ok=%v err=%v", ok, err)
		}
		unlock2()
	})
}
