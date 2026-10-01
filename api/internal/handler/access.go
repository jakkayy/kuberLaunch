package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jakkayy/kuberlauncher/api/internal/auth"
	"github.com/jakkayy/kuberlauncher/api/internal/model"
	"github.com/jakkayy/kuberlauncher/api/internal/service"
)

const projectRoleKey = "access.project_role"

type AccessHandler struct {
	svc *service.AccessService
}

func NewAccessHandler(svc *service.AccessService) *AccessHandler {
	return &AccessHandler{svc: svc}
}

// RequireProjectRole aborts unless the caller holds at least min on the
// project in the :id route parameter. Non-members get 404 so project
// existence is not revealed.
func (h *AccessHandler) RequireProjectRole(min model.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, err := h.svc.ProjectRole(c.Request.Context(), auth.UserFrom(c), c.Param("id"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !role.Valid() {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "project not found"})
			return
		}
		if !role.AtLeast(min) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "requires project role " + string(min)})
			return
		}
		c.Set(projectRoleKey, role)
		c.Next()
	}
}

// projectRole returns the role resolved by RequireProjectRole.
func projectRole(c *gin.Context) model.Role {
	if v, ok := c.Get(projectRoleKey); ok {
		return v.(model.Role)
	}
	return model.RoleNone
}

func (h *AccessHandler) Me(c *gin.Context) {
	u := auth.UserFrom(c)
	c.JSON(http.StatusOK, gin.H{"user": u, "is_admin": h.svc.IsAdmin(u)})
}

func (h *AccessHandler) ListMembers(c *gin.Context) {
	members, err := h.svc.ListProjectMembers(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	if members == nil {
		members = []model.ProjectMember{}
	}
	c.JSON(http.StatusOK, gin.H{"members": members})
}

func (h *AccessHandler) AddMember(c *gin.Context) {
	var req model.AddProjectMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.AddProjectMember(c.Request.Context(), c.Param("id"), req); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AccessHandler) RemoveMember(c *gin.Context) {
	if err := h.svc.RemoveProjectMember(c.Request.Context(), c.Param("id"), c.Param("member_id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AccessHandler) ListTeams(c *gin.Context) {
	teams, err := h.svc.ListTeams(c.Request.Context(), auth.UserFrom(c))
	if err != nil {
		respondError(c, err)
		return
	}
	if teams == nil {
		teams = []model.Team{}
	}
	c.JSON(http.StatusOK, gin.H{"teams": teams})
}

func (h *AccessHandler) CreateTeam(c *gin.Context) {
	var req model.CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	t, err := h.svc.CreateTeam(c.Request.Context(), auth.UserFrom(c), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, t)
}

func (h *AccessHandler) ListTeamMembers(c *gin.Context) {
	members, err := h.svc.ListTeamMembers(c.Request.Context(), auth.UserFrom(c), c.Param("team"))
	if err != nil {
		respondError(c, err)
		return
	}
	if members == nil {
		members = []model.TeamMember{}
	}
	c.JSON(http.StatusOK, gin.H{"members": members})
}

func (h *AccessHandler) AddTeamMember(c *gin.Context) {
	var req model.AddTeamMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.AddTeamMember(c.Request.Context(), auth.UserFrom(c), c.Param("team"), req); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AccessHandler) RemoveTeamMember(c *gin.Context) {
	if err := h.svc.RemoveTeamMember(c.Request.Context(), auth.UserFrom(c), c.Param("team"), c.Param("login")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
