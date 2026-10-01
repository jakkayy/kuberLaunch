package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jakkayy/kuberlauncher/api/internal/auth"
	"github.com/jakkayy/kuberlauncher/api/internal/generator"
	"github.com/jakkayy/kuberlauncher/api/internal/model"
	"github.com/jakkayy/kuberlauncher/api/internal/service"
)

type ProjectHandler struct {
	svc    *service.ProjectService
	access *service.AccessService
}

func NewProjectHandler(svc *service.ProjectService, access *service.AccessService) *ProjectHandler {
	return &ProjectHandler{svc: svc, access: access}
}

func (h *ProjectHandler) Create(c *gin.Context) {
	var req model.CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	p, files, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		respondError(c, err)
		return
	}
	if err := h.access.GrantOwner(c.Request.Context(), p.ID, auth.UserFrom(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "grant owner: " + err.Error()})
		return
	}
	p.MyRole = model.RoleOwner

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
	ctx := c.Request.Context()
	all, err := h.svc.List(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	visible, isAdmin, err := h.access.VisibleProjects(ctx, auth.UserFrom(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	projects := []model.Project{}
	for _, p := range all {
		if isAdmin || visible[p.ID] {
			projects = append(projects, p)
		}
	}
	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

func (h *ProjectHandler) GetByID(c *gin.Context) {
	p, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	p.MyRole = projectRole(c)
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
