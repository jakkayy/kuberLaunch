package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jakkayy/kuberlauncher/api/internal/service"
)

type MonitoringHandler struct {
	svc *service.ProjectService
}

func NewMonitoringHandler(svc *service.ProjectService) *MonitoringHandler {
	return &MonitoringHandler{svc: svc}
}

func (h *MonitoringHandler) Setup(c *gin.Context) {
	id := c.Param("id")
	dashURL, err := h.svc.SetupMonitoring(c.Request.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "project not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"grafana_url": dashURL,
		"message":     "Grafana dashboard created successfully",
	})
}
