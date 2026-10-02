//go:build integration

package repository_test

import (
	"context"
	"testing"

	"github.com/jakkayy/kuberlauncher/api/internal/model"
	"github.com/jakkayy/kuberlauncher/api/internal/repository"
	"github.com/jakkayy/kuberlauncher/api/internal/testutil"
)

func TestAccessRepository(t *testing.T) {
	conn := testutil.Postgres(t)
	ctx := context.Background()
	users := repository.NewUserRepository(conn)
	access := repository.NewAccessRepository(conn)
	projects := repository.NewProjectRepository(conn)

	alice := &model.User{GitHubID: 1, Login: "alice"}
	bob := &model.User{GitHubID: 2, Login: "bob"}
	carol := &model.User{GitHubID: 3, Login: "carol"}
	for _, u := range []*model.User{alice, bob, carol} {
		if err := users.Upsert(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	p := &model.Project{Name: "demo", Slug: "demo", Runtime: model.RuntimeGo,
		Database: model.DatabaseNone, Cache: model.CacheNone, Status: model.ProjectStatusReady}
	if err := projects.Create(ctx, p); err != nil {
		t.Fatal(err)
	}

	team := &model.Team{Slug: "platform", Name: "Platform"}
	if err := access.CreateTeam(ctx, team); err != nil {
		t.Fatal(err)
	}
	if err := access.UpsertTeamMember(ctx, team.ID, bob.ID, model.RoleViewer); err != nil {
		t.Fatal(err)
	}

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(access.UpsertProjectMember(ctx, p.ID, alice.ID, "", model.RoleOwner))
	must(access.UpsertProjectMember(ctx, p.ID, "", team.ID, model.RoleMaintainer))
	must(access.UpsertProjectMember(ctx, p.ID, bob.ID, "", model.RoleViewer))

	cases := map[*model.User]model.Role{
		alice: model.RoleOwner,
		bob:   model.RoleMaintainer, // strongest of direct viewer and team maintainer
		carol: model.RoleNone,
	}
	for u, want := range cases {
		got, err := access.ProjectRole(ctx, p.ID, u.ID)
		if err != nil || got != want {
			t.Errorf("ProjectRole(%s) = %q, %v; want %q", u.Login, got, err, want)
		}
	}

	ids, err := access.ProjectIDsForUser(ctx, bob.ID)
	if err != nil || !ids[p.ID] {
		t.Errorf("ProjectIDsForUser(bob) = %v, %v", ids, err)
	}
	if ids, _ := access.ProjectIDsForUser(ctx, carol.ID); len(ids) != 0 {
		t.Errorf("carol should see no projects, got %v", ids)
	}

	n, err := access.CountDirectOwners(ctx, p.ID)
	if err != nil || n != 1 {
		t.Errorf("CountDirectOwners = %d, %v", n, err)
	}

	teams, err := access.ListTeams(ctx, bob.ID)
	if err != nil || len(teams) != 1 || teams[0].MyRole != model.RoleViewer {
		t.Errorf("ListTeams(bob) = %+v, %v", teams, err)
	}
	if teams, _ := access.ListTeams(ctx, carol.ID); len(teams) != 0 {
		t.Errorf("ListTeams(carol) = %+v", teams)
	}
	if all, _ := access.ListTeams(ctx, ""); len(all) != 1 {
		t.Errorf("ListTeams(all) = %+v", all)
	}

	members, err := access.ListProjectMembers(ctx, p.ID)
	if err != nil || len(members) != 3 || members[0].Role != model.RoleOwner {
		t.Errorf("ListProjectMembers = %+v, %v", members, err)
	}
}
