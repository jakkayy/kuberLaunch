package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
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

// Status returns the current ArgoCD health + sync status.
// GET /api/v1/projects/:id/argocd/status
func (h *ArgoCDHandler) Status(c *gin.Context) {
	id := c.Param("id")
	status, err := h.svc.GetArgoCDStatus(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

// Rollback triggers an ArgoCD rollback to the previous revision.
// POST /api/v1/projects/:id/argocd/rollback
func (h *ArgoCDHandler) Rollback(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.RollbackArgoCD(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
