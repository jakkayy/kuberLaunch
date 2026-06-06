package argocd

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

func (c *Client) login(ctx context.Context) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"username": c.username,
		"password": c.password,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/api/v1/session", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Host = "argocd.localhost"

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("argocd login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("argocd login %d: %s", resp.StatusCode, string(b))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("argocd login decode: %w", err)
	}
	return result.Token, nil
}

// RegisterApp creates (or upserts) an ArgoCD Application for the given project.
// Returns the application name.
func (c *Client) RegisterApp(ctx context.Context, slug, repoURL string) (string, error) {
	token, err := c.login(ctx)
	if err != nil {
		return "", err
	}

	app := map[string]any{
		"metadata": map[string]any{
			"name":      slug,
			"namespace": "argocd",
		},
		"spec": map[string]any{
			"project": "default",
			"source": map[string]any{
				"repoURL":        repoURL,
				"targetRevision": "HEAD",
				"path":           "helm",
				"helm": map[string]any{
					"valueFiles": []string{"values.yaml"},
				},
			},
			"destination": map[string]any{
				"server":    "https://kubernetes.default.svc",
				"namespace": slug + "-dev",
			},
			"syncPolicy": map[string]any{
				"automated": map[string]any{
					"prune":    true,
					"selfHeal": true,
				},
				"syncOptions": []string{"CreateNamespace=true"},
			},
		},
	}

	body, _ := json.Marshal(app)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/api/v1/applications", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Host = "argocd.localhost"

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("argocd create app: %w", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)

	// 200 = created, 409 = already exists (idempotent)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		return "", fmt.Errorf("argocd create app %d: %s", resp.StatusCode, string(b))
	}

	return slug, nil
}

// AppStatus holds the health and sync state of an ArgoCD application.
type AppStatus struct {
	Health string // Healthy | Progressing | Degraded | Suspended | Missing | Unknown
	Sync   string // Synced | OutOfSync | Unknown
}

// GetAppStatus returns the current health and sync status of an ArgoCD app.
func (c *Client) GetAppStatus(ctx context.Context, appName string) (*AppStatus, error) {
	token, err := c.login(ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/api/v1/applications/"+appName, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Host = "argocd.localhost"

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("argocd get app: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("argocd get app %d: %s", resp.StatusCode, string(b))
	}

	var result struct {
		Status struct {
			Health struct {
				Status string `json:"status"`
			} `json:"health"`
			Sync struct {
				Status string `json:"status"`
			} `json:"sync"`
		} `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("argocd get app decode: %w", err)
	}

	return &AppStatus{
		Health: result.Status.Health.Status,
		Sync:   result.Status.Sync.Status,
	}, nil
}
