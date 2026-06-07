package handler

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os/exec"

	"github.com/gin-gonic/gin"
	"github.com/jakkayy/kuberlauncher/api/internal/service"
)

type LogsHandler struct {
	projectSvc *service.ProjectService
}

func NewLogsHandler(svc *service.ProjectService) *LogsHandler {
	return &LogsHandler{projectSvc: svc}
}

// Stream streams pod logs via SSE using kubectl.
// GET /api/v1/projects/:id/logs
func (h *LogsHandler) Stream(c *gin.Context) {
	id := c.Param("id")

	p, err := h.projectSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	namespace := p.Slug + "-dev"
	labelSelector := "app=" + p.Slug

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "logs",
		"-n", namespace,
		"-l", labelSelector,
		"--all-containers=true",
		"--follow=true",
		"--tail=100",
		"--prefix=true",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(c.Writer, "data: {\"error\": \"failed to start log stream\"}\n\n")
		c.Writer.Flush()
		return
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(c.Writer, "data: {\"error\": \"kubectl not available\"}\n\n")
		c.Writer.Flush()
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintf(c.Writer, "data: %s\n\n", jsonEscape(line))
		c.Writer.Flush()
	}
}

func jsonEscape(s string) string {
	b := make([]byte, 0, len(s)+2)
	b = append(b, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			b = append(b, '\\', '"')
		case '\\':
			b = append(b, '\\', '\\')
		case '\n':
			b = append(b, '\\', 'n')
		case '\r':
			b = append(b, '\\', 'r')
		case '\t':
			b = append(b, '\\', 't')
		default:
			b = append(b, c)
		}
	}
	b = append(b, '"')
	return string(b)
}
