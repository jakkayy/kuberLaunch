// Package observe provides structured request logging, request IDs,
// Prometheus metrics and health endpoints for the API.
package observe

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// SetupLogging installs a JSON slog logger as the process default. The
// standard log package is routed through it as well.
func SetupLogging(env string) {
	level := slog.LevelInfo
	if env == "development" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}

const (
	RequestIDHeader = "X-Request-ID"
	requestIDKey    = "observe.request_id"
)

// RequestID propagates an incoming X-Request-ID (or generates one) and echoes
// it in the response so logs can be correlated across web and API.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if id == "" || len(id) > 64 {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		c.Set(requestIDKey, id)
		c.Header(RequestIDHeader, id)
		c.Next()
	}
}

// RequestIDFrom returns the request ID set by RequestID.
func RequestIDFrom(c *gin.Context) string {
	return c.GetString(requestIDKey)
}

var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kuberlaunch_http_requests_total",
		Help: "HTTP requests by method, route and status code.",
	}, []string{"method", "route", "status"})

	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "kuberlaunch_http_request_duration_seconds",
		Help:    "HTTP request latency by method and route (streams excluded).",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})

	deploymentsFinished = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kuberlaunch_deployments_finished_total",
		Help: "Deployments that reached a terminal status, by environment and status.",
	}, []string{"environment", "status"})
)

// DeploymentFinished records a deployment reaching a terminal status.
func DeploymentFinished(env, status string) {
	deploymentsFinished.WithLabelValues(env, status).Inc()
}

// UserFunc returns the authenticated user's login for log lines, if any.
type UserFunc func(*gin.Context) string

// AccessLog logs one structured line per request and records HTTP metrics.
// The route template (not the raw path) is used to keep label cardinality low.
func AccessLog(user UserFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := c.Writer.Status()
		elapsed := time.Since(start)

		httpRequests.WithLabelValues(c.Request.Method, route, strconv.Itoa(status)).Inc()
		if c.Writer.Header().Get("Content-Type") != "text/event-stream" {
			httpDuration.WithLabelValues(c.Request.Method, route).Observe(elapsed.Seconds())
		}

		level := slog.LevelInfo
		switch {
		case status >= 500:
			level = slog.LevelError
		case status >= 400:
			level = slog.LevelWarn
		case route == "/healthz" || route == "/readyz" || route == "/metrics":
			level = slog.LevelDebug
		}
		attrs := []slog.Attr{
			slog.String("request_id", RequestIDFrom(c)),
			slog.String("method", c.Request.Method),
			slog.String("route", route),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Duration("duration", elapsed),
		}
		if user != nil {
			if login := user(c); login != "" {
				attrs = append(attrs, slog.String("user", login))
			}
		}
		slog.LogAttrs(c.Request.Context(), level, "http request", attrs...)
	}
}

// Register mounts /healthz (process is alive), /readyz (dependencies are
// reachable) and /metrics on r.
func Register(r *gin.Engine, db *sql.DB) {
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "database": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
}
