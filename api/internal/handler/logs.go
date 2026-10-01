package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jakkayy/kuberlauncher/api/internal/kube"
	"github.com/jakkayy/kuberlauncher/api/internal/service"
)

type LogsHandler struct {
	projectSvc *service.ProjectService
	kube       *kube.Client
}

func NewLogsHandler(svc *service.ProjectService, kube *kube.Client) *LogsHandler {
	return &LogsHandler{projectSvc: svc, kube: kube}
}

// Stream streams pod logs via SSE using the Kubernetes API.
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

	if h.kube == nil {
		fmt.Fprintf(c.Writer, "data: %s\n\n", jsonEscape("error: Kubernetes integration not configured"))
		c.Writer.Flush()
		return
	}

	ctx := c.Request.Context()
	lines := make(chan string, 100)
	errc := make(chan error, 1)
	go func() { errc <- h.kube.StreamLogs(ctx, namespace, labelSelector, 100, lines) }()

	for line := range lines {
		fmt.Fprintf(c.Writer, "data: %s\n\n", jsonEscape(line))
		c.Writer.Flush()
	}
	if err := <-errc; err != nil {
		fmt.Fprintf(c.Writer, "data: %s\n\n", jsonEscape("error: "+err.Error()))
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
