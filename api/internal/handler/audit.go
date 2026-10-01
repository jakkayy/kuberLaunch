package handler

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jakkayy/kuberlauncher/api/internal/auth"
	"github.com/jakkayy/kuberlauncher/api/internal/model"
	"github.com/jakkayy/kuberlauncher/api/internal/repository"
)

// auditActions names the routes that change state. GET routes listed here
// (the setup stream) also mutate and are audited.
var auditActions = map[string]string{
	"POST /api/v1/teams":                             "team.create",
	"PUT /api/v1/teams/:team/members":                "team.member.set",
	"DELETE /api/v1/teams/:team/members/:login":      "team.member.remove",
	"POST /api/v1/projects":                          "project.create",
	"DELETE /api/v1/projects/:id":                    "project.delete",
	"POST /api/v1/projects/:id/repo":                 "project.repo.connect",
	"POST /api/v1/projects/:id/repo/repair":          "project.repo.repair",
	"POST /api/v1/projects/:id/argocd":               "project.argocd.register",
	"POST /api/v1/projects/:id/argocd/rollback":      "project.argocd.rollback",
	"POST /api/v1/projects/:id/monitoring":           "project.monitoring.setup",
	"GET /api/v1/projects/:id/setup/stream":          "project.setup.run",
	"POST /api/v1/projects/:id/deployments":          "deployment.trigger",
	"POST /api/v1/projects/:id/secrets":              "secret.set",
	"DELETE /api/v1/projects/:id/secrets/:key":       "secret.delete",
	"PUT /api/v1/projects/:id/members":               "project.member.set",
	"DELETE /api/v1/projects/:id/members/:member_id": "project.member.remove",
	"GET /api/v1/projects/:id/logs":                  "project.logs.view",
}

type AuditHandler struct {
	repo *repository.AuditRepository
}

func NewAuditHandler(repo *repository.AuditRepository) *AuditHandler {
	return &AuditHandler{repo: repo}
}

// Middleware records every audited request after it completes, including
// rejected ones (403/404), so attempted actions are traceable. Request
// bodies are never stored; only route parameters (e.g. the secret key name).
func (h *AuditHandler) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		action, ok := auditActions[c.Request.Method+" "+c.FullPath()]
		if !ok {
			return
		}
		u := auth.UserFrom(c)
		if u == nil {
			return
		}
		details := map[string]string{}
		for _, p := range c.Params {
			if p.Key != "id" {
				details[p.Key] = p.Value
			}
		}
		if v, ok := c.Get(auditProjectKey); ok {
			details["project_id"] = v.(string)
		}
		projectID := c.Param("id")
		if projectID == "" {
			projectID = details["project_id"]
		}
		if len(c.Errors) > 0 {
			details["error"] = strings.TrimSpace(c.Errors.String())
		}
		e := &model.AuditEntry{
			ActorID: u.ID, ActorLogin: u.Login, Action: action,
			ProjectID: projectID, Status: c.Writer.Status(), Details: details,
		}
		// Record even if the client already disconnected.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 5*time.Second)
		defer cancel()
		if err := h.repo.Insert(ctx, e); err != nil {
			log.Printf("audit: %v", err)
		}
	}
}

const auditProjectKey = "audit.project_id"

// setAuditProject attaches a project ID created during the request (project.create).
func setAuditProject(c *gin.Context, id string) { c.Set(auditProjectKey, id) }

const auditPageSize = 100

func (h *AuditHandler) ListForProject(c *gin.Context) {
	h.list(c, c.Param("id"))
}

// ListAll is for platform admins.
func (h *AuditHandler) ListAll(c *gin.Context) {
	h.list(c, "")
}

func (h *AuditHandler) list(c *gin.Context, projectID string) {
	entries, err := h.repo.List(c.Request.Context(), projectID, auditPageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if entries == nil {
		entries = []model.AuditEntry{}
	}
	c.JSON(http.StatusOK, gin.H{"entries": entries})
}
