package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jakkayy/kuberlauncher/api/internal/service"
)

type RepoHandler struct {
	svc *service.ProjectService
}

func NewRepoHandler(svc *service.ProjectService) *RepoHandler {
	return &RepoHandler{svc: svc}
}

func (h *RepoHandler) Connect(c *gin.Context) {
	id := c.Param("id")
	repoURL, err := h.svc.ConnectGitHub(c.Request.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "project not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"repo_url": repoURL,
		"message": "GitHub repo created and files pushed successfully",
	})
}
