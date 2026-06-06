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
