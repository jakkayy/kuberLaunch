package argocd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jakkayy/kuberlauncher/api/internal/httpx"
)

type Client struct {
	baseURL  string
	apiToken string // account API token; preferred over username/password
	username string
	password string
	http     *http.Client

	mu      sync.Mutex
	session string    // cached session token from username/password login
	expires time.Time // session expiry from its JWT exp claim
}

// New creates an ArgoCD client. When apiToken is set it is used as-is
// (see k8s/platform/argocd-account.yaml for a least-privilege account);
// otherwise the client logs in with username/password and caches the session.
func New(url, apiToken, username, password string) *Client {
	return &Client{
		baseURL:  url,
		apiToken: apiToken,
		username: username,
		password: password,
		http:     httpx.Client(30 * time.Second),
	}
}

// sessionMargin re-logs in this long before a cached session expires.
const sessionMargin = time.Minute

// token returns the bearer token for API calls. forceLogin discards a cached
// session (after the server rejected it).
func (c *Client) token(ctx context.Context, forceLogin bool) (string, error) {
	if c.apiToken != "" {
		return c.apiToken, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !forceLogin && c.session != "" && time.Now().Before(c.expires.Add(-sessionMargin)) {
		return c.session, nil
	}
	tok, err := c.login(ctx)
	if err != nil {
		return "", err
	}
	c.session, c.expires = tok, jwtExpiry(tok)
	return tok, nil
}

// jwtExpiry reads the exp claim of an ArgoCD session token without verifying
// it (the server does that); unknown expiry means "re-login in 5 minutes".
func jwtExpiry(token string) time.Time {
	fallback := time.Now().Add(5 * time.Minute)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return fallback
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fallback
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Exp == 0 {
		return fallback
	}
	return time.Unix(claims.Exp, 0)
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

// EnsureProject creates or updates an AppProject named slug that may only
// deploy from repoURL into the slug-* namespaces of the local cluster and may
// create no cluster-scoped resources other than those namespaces.
func (c *Client) EnsureProject(ctx context.Context, slug, repoURL string) error {
	project := map[string]any{
		"metadata": map[string]any{"name": slug, "namespace": "argocd"},
		"spec": map[string]any{
			"description": "kuberLaunch project " + slug,
			"sourceRepos": []string{repoURL},
			"destinations": []map[string]string{
				{"server": "https://kubernetes.default.svc", "namespace": slug + "-*"},
			},
			"clusterResourceWhitelist": []map[string]string{
				{"group": "", "kind": "Namespace"},
			},
		},
	}
	_, err := c.call(ctx, http.MethodPost, "/api/v1/projects",
		map[string]any{"project": project, "upsert": true}, "create project", http.StatusOK)
	return err
}

// DeleteProject removes the AppProject; its applications must be deleted first.
func (c *Client) DeleteProject(ctx context.Context, slug string) error {
	_, err := c.call(ctx, http.MethodDelete, "/api/v1/projects/"+slug, nil, "delete project",
		http.StatusOK, http.StatusNotFound)
	return err
}

// AppSpec describes one environment's Application.
type AppSpec struct {
	Name        string // ArgoCD application name, e.g. demo-dev
	Project     string // AppProject (the project slug)
	RepoURL     string
	ReleaseName string   // Helm release; identical across envs so object names are stable
	ValueFiles  []string // relative to the chart path
	Namespace   string
	AutoSync    bool
}

// RegisterApp creates or updates an Application. Call EnsureProject first.
func (c *Client) RegisterApp(ctx context.Context, spec AppSpec) error {
	syncPolicy := map[string]any{"syncOptions": []string{"CreateNamespace=true"}}
	if spec.AutoSync {
		syncPolicy["automated"] = map[string]any{"prune": true, "selfHeal": true}
	}
	app := map[string]any{
		"metadata": map[string]any{"name": spec.Name, "namespace": "argocd"},
		"spec": map[string]any{
			"project": spec.Project,
			"source": map[string]any{
				"repoURL":        spec.RepoURL,
				"targetRevision": "HEAD",
				"path":           "helm",
				"helm": map[string]any{
					"releaseName": spec.ReleaseName,
					"valueFiles":  spec.ValueFiles,
				},
			},
			"destination": map[string]any{
				"server":    "https://kubernetes.default.svc",
				"namespace": spec.Namespace,
			},
			"syncPolicy": syncPolicy,
		},
	}
	_, err := c.call(ctx, http.MethodPost, "/api/v1/applications?upsert=true", app, "create app", http.StatusOK)
	return err
}

// SyncApp starts a sync of the app to the latest Git revision.
func (c *Client) SyncApp(ctx context.Context, appName string) error {
	_, err := c.call(ctx, http.MethodPost, "/api/v1/applications/"+appName+"/sync",
		map[string]any{"prune": true}, "sync app", http.StatusOK)
	return err
}

// OrphanApp deletes an Application but leaves its Kubernetes resources
// running, so another Application can adopt them without downtime.
func (c *Client) OrphanApp(ctx context.Context, appName string) error {
	_, err := c.call(ctx, http.MethodDelete, "/api/v1/applications/"+appName+"?cascade=false", nil,
		"orphan app", http.StatusOK, http.StatusNotFound)
	return err
}

// call sends an authenticated JSON request and returns the body when the
// response status is one of ok. A 401 with a cached session triggers one
// re-login and retry.
func (c *Client) call(ctx context.Context, method, path string, body any, what string, ok ...int) ([]byte, error) {
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	}
	status, b, err := c.send(ctx, method, path, payload, false)
	if err == nil && status == http.StatusUnauthorized && c.apiToken == "" {
		status, b, err = c.send(ctx, method, path, payload, true)
	}
	if err != nil {
		return nil, fmt.Errorf("argocd %s: %w", what, err)
	}
	for _, code := range ok {
		if status == code {
			return b, nil
		}
	}
	return nil, fmt.Errorf("argocd %s %d: %s", what, status, string(b))
}

func (c *Client) send(ctx context.Context, method, path string, payload []byte, forceLogin bool) (int, []byte, error) {
	token, err := c.token(ctx, forceLogin)
	if err != nil {
		return 0, nil, err
	}
	var r io.Reader
	if payload != nil {
		r = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, r)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b, nil
}

// DeleteApp removes an ArgoCD Application (cascade deletes all resources).
func (c *Client) DeleteApp(ctx context.Context, appName string) error {
	_, err := c.call(ctx, http.MethodDelete, "/api/v1/applications/"+appName+"?cascade=true", nil,
		"delete app", http.StatusOK, http.StatusNotFound)
	return err
}

// RollbackApp disables auto-sync (ArgoCD refuses to roll back while it is on)
// then rolls back to the previous revision.
func (c *Client) RollbackApp(ctx context.Context, appName string) error {
	patch := map[string]any{"spec": map[string]any{
		"syncPolicy": map[string]any{"syncOptions": []string{"CreateNamespace=true"}},
	}}
	if _, err := c.call(ctx, http.MethodPatch, "/api/v1/applications/"+appName+"?patchType=merge", patch,
		"disable auto-sync", http.StatusOK); err != nil {
		return err
	}
	_, err := c.call(ctx, http.MethodPost, "/api/v1/applications/"+appName+"/rollback",
		map[string]any{"id": 0, "dryRun": false, "prune": false}, "rollback", http.StatusOK)
	return err
}

// AppStatus holds the health and sync state of an ArgoCD application.
type AppStatus struct {
	Health   string   // Healthy | Progressing | Degraded | Suspended | Missing | Unknown
	Sync     string   // Synced | OutOfSync | Unknown
	Revision string   // git revision ArgoCD last synced
	Images   []string // container images currently running in the app
}

// GetAppStatus returns the current health and sync status of an ArgoCD app.
func (c *Client) GetAppStatus(ctx context.Context, appName string) (*AppStatus, error) {
	b, err := c.call(ctx, http.MethodGet, "/api/v1/applications/"+appName, nil, "get app", http.StatusOK)
	if err != nil {
		return nil, err
	}
	var result struct {
		Status struct {
			Health struct {
				Status string `json:"status"`
			} `json:"health"`
			Sync struct {
				Status   string `json:"status"`
				Revision string `json:"revision"`
			} `json:"sync"`
			Summary struct {
				Images []string `json:"images"`
			} `json:"summary"`
		} `json:"status"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("argocd get app decode: %w", err)
	}
	return &AppStatus{
		Health:   result.Status.Health.Status,
		Sync:     result.Status.Sync.Status,
		Revision: result.Status.Sync.Revision,
		Images:   result.Status.Summary.Images,
	}, nil
}
