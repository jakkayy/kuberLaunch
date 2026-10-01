package model

import "time"

// Role is a project or team permission level. Higher roles include lower ones.
type Role string

const (
	RoleNone       Role = ""
	RoleViewer     Role = "viewer"
	RoleMaintainer Role = "maintainer"
	RoleOwner      Role = "owner"
)

func (r Role) rank() int {
	switch r {
	case RoleViewer:
		return 1
	case RoleMaintainer:
		return 2
	case RoleOwner:
		return 3
	}
	return 0
}

// AtLeast reports whether r grants everything min grants.
func (r Role) AtLeast(min Role) bool { return r.rank() >= min.rank() }

func (r Role) Valid() bool { return r.rank() > 0 }

type Team struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	MyRole    Role      `json:"my_role,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type TeamMember struct {
	UserID string `json:"user_id"`
	Login  string `json:"login"`
	Role   Role   `json:"role"`
}

// ProjectMember grants Role on a project to either a user or a team.
type ProjectMember struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	UserID    string    `json:"user_id,omitempty"`
	Login     string    `json:"login,omitempty"`
	TeamID    string    `json:"team_id,omitempty"`
	TeamSlug  string    `json:"team_slug,omitempty"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type AddProjectMemberRequest struct {
	Login string `json:"login"`
	Team  string `json:"team"` // team slug
	Role  Role   `json:"role" binding:"required,oneof=viewer maintainer owner"`
}

type CreateTeamRequest struct {
	Slug string `json:"slug" binding:"required,min=2,max=40"`
	Name string `json:"name" binding:"required,min=2,max=80"`
}

type AddTeamMemberRequest struct {
	Login string `json:"login" binding:"required"`
	Role  Role   `json:"role" binding:"required,oneof=viewer maintainer owner"`
}
