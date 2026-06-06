package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jakkayy/kuberlauncher/api/internal/model"
	"github.com/jakkayy/kuberlauncher/api/internal/service"
)

type DeploymentHandler struct {
	svc        *service.DeploymentService
	projectSvc *service.ProjectService
}

func NewDeploymentHandler(svc *service.DeploymentService, projectSvc *service.ProjectService) *DeploymentHandler {
	return &DeploymentHandler{svc: svc, projectSvc: projectSvc}
}

func (h *DeploymentHandler) Trigger(c *gin.Context) {
	projectID := c.Param("id")
	var req model.TriggerDeploymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	d, err := h.svc.Trigger(c.Request.Context(), projectID, req.Branch)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "project not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, d)
}

func (h *DeploymentHandler) List(c *gin.Context) {
	projectID := c.Param("id")
	deployments, err := h.svc.ListByProject(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if deployments == nil {
		deployments = []model.Deployment{}
	}
	c.JSON(http.StatusOK, gin.H{"deployments": deployments})
}

func (h *DeploymentHandler) Get(c *gin.Context) {
	d, err := h.svc.GetByID(c.Request.Context(), c.Param("dep_id"))
	if err != nil || d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "deployment not found"})
		return
	}
	c.JSON(http.StatusOK, d)
}

// Stream sends SSE events with deployment status updates every 5 seconds
// until the deployment reaches a terminal state or the client disconnects.
func (h *DeploymentHandler) Stream(c *gin.Context) {
	projectID := c.Param("id")
	depID := c.Param("dep_id")

	p, err := h.projectSvc.GetByID(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	ctx := c.Request.Context()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	sendEvent := func(status model.DeploymentStatus) {
		data, _ := json.Marshal(map[string]string{"status": string(status)})
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		c.Writer.Flush()
	}

	// Send initial status immediately
	d, _ := h.svc.GetByID(ctx, depID)
	if d != nil {
		sendEvent(d.Status)
		if d.Status.IsTerminal() {
			return
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status, err := h.svc.PollStatus(ctx, depID, p.Slug, p.ArgocdApp)
			if err != nil {
				sendEvent(model.DeploymentStatusFailed)
				return
			}
			sendEvent(status)
			if model.DeploymentStatus(status).IsTerminal() {
				return
			}
		}
	}
}
