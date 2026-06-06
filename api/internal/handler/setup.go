package handler

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/jakkayy/kuberlauncher/api/internal/service"
)

type SetupHandler struct {
	svc *service.ProjectService
}

func NewSetupHandler(svc *service.ProjectService) *SetupHandler {
	return &SetupHandler{svc: svc}
}

// Stream runs OneClickSetup and streams progress as SSE events.
func (h *SetupHandler) Stream(c *gin.Context) {
	id := c.Param("id")

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	ctx := c.Request.Context()
	ch := h.svc.OneClickSetup(ctx, id)

	for progress := range ch {
		data, _ := json.Marshal(progress)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		c.Writer.Flush()
	}
}
