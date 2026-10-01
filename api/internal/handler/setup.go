package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jakkayy/kuberlauncher/api/internal/service"
)

// streamMaxDuration caps a setup stream that is opened without a run in progress.
const streamMaxDuration = 15 * time.Minute

type SetupHandler struct {
	svc *service.SetupService
}

func NewSetupHandler(svc *service.SetupService) *SetupHandler {
	return &SetupHandler{svc: svc}
}

// Start begins (or resumes) the Golden Path setup in the background.
// POST /api/v1/projects/:id/setup
func (h *SetupHandler) Start(c *gin.Context) {
	if err := h.svc.Start(c.Request.Context(), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	h.Status(c)
}

// Status returns the persisted state of each setup step.
// GET /api/v1/projects/:id/setup
func (h *SetupHandler) Status(c *gin.Context) {
	state, err := h.svc.State(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	status := http.StatusOK
	if c.Request.Method == http.MethodPost {
		status = http.StatusAccepted
	}
	c.JSON(status, state)
}

// Stream sends the setup state as SSE whenever it changes, until no step is
// running. Closing the stream does not stop the setup.
// GET /api/v1/projects/:id/setup/stream
func (h *SetupHandler) Stream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	ctx, cancel := context.WithTimeout(c.Request.Context(), streamMaxDuration)
	defer cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var last string
	for {
		state, err := h.svc.State(ctx, c.Param("id"))
		if err != nil {
			return
		}
		data, _ := json.Marshal(state)
		if string(data) != last {
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			c.Writer.Flush()
			last = string(data)
		}
		if state.Finished() {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
