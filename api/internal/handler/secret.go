package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jakkayy/kuberlauncher/api/internal/service"
)

type SecretHandler struct {
	svc *service.SecretService
}

func NewSecretHandler(svc *service.SecretService) *SecretHandler {
	return &SecretHandler{svc: svc}
}

// Set creates or updates a secret key for a project.
// POST /api/v1/projects/:id/secrets
func (h *SecretHandler) Set(c *gin.Context) {
	projectID := c.Param("id")

	var req struct {
		Key   string `json:"key" binding:"required,min=1,max=128"`
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.Set(c.Request.Context(), projectID, req.Key, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ListKeys returns all secret key names (not values) for a project.
// GET /api/v1/projects/:id/secrets
func (h *SecretHandler) ListKeys(c *gin.Context) {
	projectID := c.Param("id")

	keys, err := h.svc.ListKeys(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"keys": keys})
}

// Delete removes a secret key from a project.
// DELETE /api/v1/projects/:id/secrets/:key
func (h *SecretHandler) Delete(c *gin.Context) {
	projectID := c.Param("id")
	key := c.Param("key")

	if err := h.svc.Delete(c.Request.Context(), projectID, key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
