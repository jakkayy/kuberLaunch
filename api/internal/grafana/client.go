package grafana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL  string
	username string
	password string
	http     *http.Client
}

func New(url, username, password string) *Client {
	return &Client{
		baseURL:  url,
		username: username,
		password: password,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, r)
	if err != nil {
		return nil, 0, err
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")
	req.Host = "grafana.localhost"

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return b, resp.StatusCode, nil
}

// EnsureFolder creates a folder for the project if it doesn't exist.
func (c *Client) EnsureFolder(ctx context.Context, slug string) (string, error) {
	uid := "kuberlauncher-" + slug

	// ลองดึง folder ที่มีอยู่
	_, status, err := c.do(ctx, http.MethodGet, "/api/folders/"+uid, nil)
	if err == nil && status == http.StatusOK {
		return uid, nil
	}

	// สร้างใหม่
	body := map[string]string{"uid": uid, "title": slug}
	b, status, err := c.do(ctx, http.MethodPost, "/api/folders", body)
	if err != nil {
		return "", fmt.Errorf("create folder: %w", err)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("create folder %d: %s", status, string(b))
	}
	return uid, nil
}

// CreateDashboard creates a pre-built dashboard for the project.
// Returns the dashboard URL.
func (c *Client) CreateDashboard(ctx context.Context, slug, folderUID string) (string, error) {
	dashboard := buildDashboard(slug)
	payload := map[string]any{
		"dashboard": dashboard,
		"folderUid": folderUID,
		"overwrite": true,
		"message":   "created by kuberLaunch",
	}

	b, status, err := c.do(ctx, http.MethodPost, "/api/dashboards/db", payload)
	if err != nil {
		return "", fmt.Errorf("create dashboard: %w", err)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("create dashboard %d: %s", status, string(b))
	}

	var result struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return "", err
	}
	return result.URL, nil
}

// buildDashboard returns a Grafana dashboard JSON with CPU, Memory, and HTTP panels
// scoped to the given project slug (kubernetes namespace = slug-dev).
func buildDashboard(slug string) map[string]any {
	namespace := slug + "-dev"
	return map[string]any{
		"title": slug + " — Overview",
		"uid":   "kuberlauncher-dash-" + slug,
		"tags":  []string{"kuberlauncher", slug},
		"time":  map[string]string{"from": "now-1h", "to": "now"},
		"refresh": "30s",
		"panels": []map[string]any{
			panel(1, "CPU Usage", 0, 0,
				`sum(rate(container_cpu_usage_seconds_total{namespace="`+namespace+`"}[5m])) by (pod)`),
			panel(2, "Memory Usage (MB)", 12, 0,
				`sum(container_memory_working_set_bytes{namespace="`+namespace+`"}) by (pod) / 1024 / 1024`),
			panel(3, "HTTP Requests/sec", 0, 8,
				`sum(rate(http_requests_total{namespace="`+namespace+`"}[1m])) by (pod)`),
		},
	}
}

func panel(id int, title string, x, y int, expr string) map[string]any {
	return map[string]any{
		"id":    id,
		"title": title,
		"type":  "timeseries",
		"gridPos": map[string]int{
			"x": x, "y": y, "w": 12, "h": 8,
		},
		"targets": []map[string]any{
			{
				"datasource": map[string]string{"type": "prometheus", "uid": "prometheus"},
				"expr":       expr,
				"legendFormat": "{{pod}}",
			},
		},
	}
}
