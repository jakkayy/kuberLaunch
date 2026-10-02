package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jakkayy/kuberlauncher/api/internal/model"
	"github.com/jakkayy/kuberlauncher/api/internal/repository"
)

// AccessService answers "what may this user do?" for projects and teams.
// Platform admins (PLATFORM_ADMINS) are treated as owners everywhere.
type AccessService struct {
	repo   *repository.AccessRepository
	users  *repository.UserRepository
	admins map[string]bool
}

func NewAccessService(repo *repository.AccessRepository, users *repository.UserRepository, adminLogins []string) *AccessService {
	admins := map[string]bool{}
	for _, l := range adminLogins {
		if l = strings.ToLower(strings.TrimSpace(l)); l != "" {
			admins[l] = true
		}
	}
	return &AccessService{repo: repo, users: users, admins: admins}
}

func (s *AccessService) IsAdmin(u *model.User) bool {
	return u != nil && s.admins[strings.ToLower(u.Login)]
}

func (s *AccessService) ProjectRole(ctx context.Context, u *model.User, projectID string) (model.Role, error) {
	if u == nil {
		return model.RoleNone, nil
	}
	if s.IsAdmin(u) {
		return model.RoleOwner, nil
	}
	return s.repo.ProjectRole(ctx, projectID, u.ID)
}

// VisibleProjects returns the project IDs u may list; all is true for admins.
func (s *AccessService) VisibleProjects(ctx context.Context, u *model.User) (ids map[string]bool, all bool, err error) {
	if s.IsAdmin(u) {
		return nil, true, nil
	}
	ids, err = s.repo.ProjectIDsForUser(ctx, u.ID)
	return ids, false, err
}

func (s *AccessService) GrantOwner(ctx context.Context, projectID string, u *model.User) error {
	return s.repo.UpsertProjectMember(ctx, projectID, u.ID, "", model.RoleOwner)
}

func (s *AccessService) ListProjectMembers(ctx context.Context, projectID string) ([]model.ProjectMember, error) {
	return s.repo.ListProjectMembers(ctx, projectID)
}

func (s *AccessService) AddProjectMember(ctx context.Context, projectID string, req model.AddProjectMemberRequest) error {
	if (req.Login == "") == (req.Team == "") {
		return fmt.Errorf("%w: specify exactly one of login or team", ErrInvalid)
	}
	if req.Login != "" {
		u, err := s.users.FindByLogin(ctx, req.Login)
		if err != nil {
			return err
		}
		if u == nil {
			return fmt.Errorf("%w: user %q has never signed in to kuberLaunch", ErrInvalid, req.Login)
		}
		return s.repo.UpsertProjectMember(ctx, projectID, u.ID, "", req.Role)
	}
	t, err := s.repo.FindTeamBySlug(ctx, req.Team)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("%w: team %q not found", ErrInvalid, req.Team)
	}
	return s.repo.UpsertProjectMember(ctx, projectID, "", t.ID, req.Role)
}

func (s *AccessService) RemoveProjectMember(ctx context.Context, projectID, memberID string) error {
	m, err := s.repo.FindProjectMember(ctx, projectID, memberID)
	if err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("%w: member", ErrNotFound)
	}
	if m.Role == model.RoleOwner {
		n, err := s.repo.CountDirectOwners(ctx, projectID)
		if err != nil {
			return err
		}
		if n <= 1 {
			return fmt.Errorf("%w: cannot remove the last owner", ErrConflict)
		}
	}
	return s.repo.DeleteProjectMember(ctx, projectID, memberID)
}

var teamSlugRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

func (s *AccessService) CreateTeam(ctx context.Context, u *model.User, req model.CreateTeamRequest) (*model.Team, error) {
	if !teamSlugRe.MatchString(req.Slug) {
		return nil, fmt.Errorf("%w: team slug must be lowercase letters, digits and hyphens", ErrInvalid)
	}
	existing, err := s.repo.FindTeamBySlug(ctx, req.Slug)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("%w: team %q already exists", ErrConflict, req.Slug)
	}
	t := &model.Team{Slug: req.Slug, Name: req.Name}
	if err := s.repo.CreateTeam(ctx, t); err != nil {
		return nil, err
	}
	if err := s.repo.UpsertTeamMember(ctx, t.ID, u.ID, model.RoleOwner); err != nil {
		return nil, err
	}
	t.MyRole = model.RoleOwner
	return t, nil
}

func (s *AccessService) ListTeams(ctx context.Context, u *model.User) ([]model.Team, error) {
	if s.IsAdmin(u) {
		return s.repo.ListTeams(ctx, "")
	}
	return s.repo.ListTeams(ctx, u.ID)
}

// team loads a team and checks that u holds at least min on it.
func (s *AccessService) team(ctx context.Context, u *model.User, slug string, min model.Role) (*model.Team, error) {
	t, err := s.repo.FindTeamBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, fmt.Errorf("%w: team", ErrNotFound)
	}
	if s.IsAdmin(u) {
		return t, nil
	}
	role, err := s.repo.TeamRole(ctx, t.ID, u.ID)
	if err != nil {
		return nil, err
	}
	if !role.Valid() {
		return nil, fmt.Errorf("%w: team", ErrNotFound)
	}
	if !role.AtLeast(min) {
		return nil, fmt.Errorf("%w: requires team role %s", ErrForbidden, min)
	}
	return t, nil
}

func (s *AccessService) ListTeamMembers(ctx context.Context, u *model.User, slug string) ([]model.TeamMember, error) {
	t, err := s.team(ctx, u, slug, model.RoleViewer)
	if err != nil {
		return nil, err
	}
	return s.repo.ListTeamMembers(ctx, t.ID)
}

func (s *AccessService) AddTeamMember(ctx context.Context, u *model.User, slug string, req model.AddTeamMemberRequest) error {
	t, err := s.team(ctx, u, slug, model.RoleOwner)
	if err != nil {
		return err
	}
	member, err := s.users.FindByLogin(ctx, req.Login)
	if err != nil {
		return err
	}
	if member == nil {
		return fmt.Errorf("%w: user %q has never signed in to kuberLaunch", ErrInvalid, req.Login)
	}
	return s.repo.UpsertTeamMember(ctx, t.ID, member.ID, req.Role)
}

func (s *AccessService) RemoveTeamMember(ctx context.Context, u *model.User, slug, login string) error {
	t, err := s.team(ctx, u, slug, model.RoleOwner)
	if err != nil {
		return err
	}
	member, err := s.users.FindByLogin(ctx, login)
	if err != nil {
		return err
	}
	if member == nil {
		return fmt.Errorf("%w: user", ErrNotFound)
	}
	return s.repo.DeleteTeamMember(ctx, t.ID, member.ID)
}
