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
		respondError(c, err)
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

// Stream sends SSE events whenever the deployment status changes, until it
// reaches a terminal state or the client disconnects. Status itself is advanced
// by the server-side watcher; this handler only reads it.
func (h *DeploymentHandler) Stream(c *gin.Context) {
	depID := c.Param("dep_id")

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	ctx := c.Request.Context()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	sendEvent := func(status model.DeploymentStatus) {
		data, _ := json.Marshal(map[string]string{"status": string(status)})
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		c.Writer.Flush()
	}

	var last model.DeploymentStatus
	for {
		d, err := h.svc.GetByID(ctx, depID)
		if err != nil || d == nil {
			sendEvent(model.DeploymentStatusFailed)
			return
		}
		if d.Status != last {
			sendEvent(d.Status)
			last = d.Status
		}
		if d.Status.IsTerminal() {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
