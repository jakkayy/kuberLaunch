package observe

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestRequestIDPropagatesOrGenerates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, RequestIDFrom(c)) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(RequestIDHeader, "abc123")
	r.ServeHTTP(w, req)
	if w.Body.String() != "abc123" || w.Header().Get(RequestIDHeader) != "abc123" {
		t.Fatalf("propagated id = %q / %q", w.Body.String(), w.Header().Get(RequestIDHeader))
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if len(w.Body.String()) != 16 {
		t.Fatalf("generated id = %q", w.Body.String())
	}
}

func TestMetricsEndpointExposesRequestCounter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AccessLog(nil))
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/ping", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ping", nil))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(w.Body.String(), `kuberlaunch_http_requests_total{method="GET",route="/ping",status="204"} 1`) {
		t.Fatalf("metrics output missing request counter:\n%s", w.Body.String())
	}
}
