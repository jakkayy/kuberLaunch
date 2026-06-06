package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jakkayy/kuberlauncher/api/internal/generator"
	"github.com/jakkayy/kuberlauncher/api/internal/model"
	"github.com/jakkayy/kuberlauncher/api/internal/service"
)

type ProjectHandler struct {
	svc *service.ProjectService
}

func NewProjectHandler(svc *service.ProjectService) *ProjectHandler {
	return &ProjectHandler{svc: svc}
}

func (h *ProjectHandler) Create(c *gin.Context) {
	var req model.CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	p, files, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	filePaths := make([]string, len(files))
	for i, f := range files {
		filePaths[i] = f.Path
	}

	c.JSON(http.StatusCreated, gin.H{
		"project":         p,
		"files_generated": filePaths,
		"download_url":    fmt.Sprintf("/api/v1/projects/%s/download", p.ID),
		"preview_url":     fmt.Sprintf("/api/v1/projects/%s/preview", p.ID),
	})
}

func (h *ProjectHandler) List(c *gin.Context) {
	projects, err := h.svc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if projects == nil {
		projects = []model.Project{}
	}
	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

func (h *ProjectHandler) GetByID(c *gin.Context) {
	p, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *ProjectHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ProjectHandler) Download(c *gin.Context) {
	files, err := h.svc.Preview(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	zipBytes, err := generator.ToZip(files)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create zip"})
		return
	}

	projectID := c.Param("id")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-templates.zip"`, projectID))
	c.Data(http.StatusOK, "application/zip", zipBytes)
}

func (h *ProjectHandler) Preview(c *gin.Context) {
	files, err := h.svc.Preview(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	type filePreview struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	previews := make([]filePreview, len(files))
	for i, f := range files {
		previews[i] = filePreview{Path: f.Path, Content: f.Content}
	}
	c.JSON(http.StatusOK, gin.H{"files": previews})
}
