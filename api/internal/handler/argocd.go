package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jakkayy/kuberlauncher/api/internal/model"
	"github.com/jakkayy/kuberlauncher/api/internal/service"
)

type ArgoCDHandler struct {
	svc *service.ProjectService
}

func NewArgoCDHandler(svc *service.ProjectService) *ArgoCDHandler {
	return &ArgoCDHandler{svc: svc}
}

func (h *ArgoCDHandler) Register(c *gin.Context) {
	id := c.Param("id")
	appName, err := h.svc.RegisterArgoCD(c.Request.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "project not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"argocd_app": appName,
		"message":    "ArgoCD application registered successfully",
	})
}

// Environments returns every environment with its live ArgoCD status.
// GET /api/v1/projects/:id/environments
func (h *ArgoCDHandler) Environments(c *gin.Context) {
	envs, err := h.svc.Environments(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"environments": envs})
}

// Rollback rolls one environment back to its previous revision.
// POST /api/v1/projects/:id/environments/:env/rollback
func (h *ArgoCDHandler) Rollback(c *gin.Context) {
	if err := h.svc.RollbackArgoCD(c.Request.Context(), c.Param("id"), model.EnvName(c.Param("env"))); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
