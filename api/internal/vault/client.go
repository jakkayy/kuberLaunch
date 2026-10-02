package vault

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jakkayy/kuberlauncher/api/internal/httpx"
)

type Client struct {
	baseURL string
	http    *http.Client

	// Static token auth (local dev).
	staticToken string

	// Kubernetes auth: exchange the pod's service account JWT for a token.
	k8s *KubernetesAuth

	mu      sync.Mutex
	token   string
	expires time.Time
}

// KubernetesAuth configures Vault's Kubernetes auth method.
type KubernetesAuth struct {
	Mount     string // auth mount path, e.g. "kubernetes"
	Role      string // Vault role bound to the API's service account
	TokenPath string // service account JWT, e.g. /var/run/secrets/kubernetes.io/serviceaccount/token
}

// New creates a client that authenticates with a static token.
func New(url, token string) *Client {
	return &Client{
		baseURL:     url,
		staticToken: token,
		http:        httpx.Client(15 * time.Second),
	}
}

// NewKubernetes creates a client that logs in with Kubernetes auth and
// renews its token when it expires.
func NewKubernetes(url string, auth KubernetesAuth) *Client {
	return &Client{
		baseURL: url,
		k8s:     &auth,
		http:    httpx.Client(15 * time.Second),
	}
}

// renewMargin re-authenticates this long before the Vault token expires.
const renewMargin = time.Minute

func (c *Client) authToken(ctx context.Context, forceLogin bool) (string, error) {
	if c.k8s == nil {
		return c.staticToken, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !forceLogin && c.token != "" && time.Now().Before(c.expires.Add(-renewMargin)) {
		return c.token, nil
	}

	jwt, err := os.ReadFile(c.k8s.TokenPath)
	if err != nil {
		return "", fmt.Errorf("read service account token: %w", err)
	}
	body, _ := json.Marshal(map[string]string{"role": c.k8s.Role, "jwt": strings.TrimSpace(string(jwt))})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/v1/auth/"+c.k8s.Mount+"/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("vault login: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("vault login %d: %s", resp.StatusCode, string(b))
	}
	var result struct {
		Auth struct {
			ClientToken   string `json:"client_token"`
			LeaseDuration int    `json:"lease_duration"`
		} `json:"auth"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("vault login decode: %w", err)
	}
	c.token = result.Auth.ClientToken
	c.expires = time.Now().Add(time.Duration(result.Auth.LeaseDuration) * time.Second)
	return c.token, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	b, status, err := c.doOnce(ctx, method, path, body, false)
	if err == nil && status == http.StatusForbidden && c.k8s != nil {
		// Token may have been revoked or expired early; log in again once.
		return c.doOnce(ctx, method, path, body, true)
	}
	return b, status, err
}

func (c *Client) doOnce(ctx context.Context, method, path string, body any, forceLogin bool) ([]byte, int, error) {
	token, err := c.authToken(ctx, forceLogin)
	if err != nil {
		return nil, 0, err
	}
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, r)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("X-Vault-Token", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return b, resp.StatusCode, nil
}

// ProjectPolicyName is the Vault policy and Kubernetes auth role created per project.
func ProjectPolicyName(slug string) string { return "kuberlaunch-" + slug }

// ProjectServiceAccount is the service account the generated Helm chart
// creates for External Secrets to authenticate to Vault.
func ProjectServiceAccount(slug string) string { return slug + "-vault" }

// EnsureProjectAccess creates (or updates) a read-only policy for the
// project's secret path and a Kubernetes auth role that grants it only to the
// project's service account in the given namespaces.
func (c *Client) EnsureProjectAccess(ctx context.Context, authMount, slug string, namespaces []string) error {
	name := ProjectPolicyName(slug)
	policy := fmt.Sprintf("path %q {\n  capabilities = [\"read\"]\n}\n", "secret/data/kuberlauncher/"+slug)
	if err := c.expectOK(ctx, http.MethodPut, "/v1/sys/policies/acl/"+name,
		map[string]string{"policy": policy}, "write policy"); err != nil {
		return err
	}
	return c.expectOK(ctx, http.MethodPost, "/v1/auth/"+authMount+"/role/"+name, map[string]any{
		"bound_service_account_names":      []string{ProjectServiceAccount(slug)},
		"bound_service_account_namespaces": namespaces,
		"token_policies":                   []string{name},
		"token_ttl":                        "1h",
	}, "write kubernetes role")
}

// DeleteProjectAccess removes the project's Kubernetes auth role and policy.
func (c *Client) DeleteProjectAccess(ctx context.Context, authMount, slug string) error {
	name := ProjectPolicyName(slug)
	if err := c.expectOK(ctx, http.MethodDelete, "/v1/auth/"+authMount+"/role/"+name, nil, "delete kubernetes role"); err != nil {
		return err
	}
	return c.expectOK(ctx, http.MethodDelete, "/v1/sys/policies/acl/"+name, nil, "delete policy")
}

func (c *Client) expectOK(ctx context.Context, method, path string, body any, what string) error {
	b, status, err := c.do(ctx, method, path, body)
	if err != nil {
		return fmt.Errorf("vault %s: %w", what, err)
	}
	switch {
	case status == http.StatusOK, status == http.StatusNoContent:
		return nil
	case method == http.MethodDelete && status == http.StatusNotFound:
		return nil // already gone
	}
	return fmt.Errorf("vault %s %d: %s", what, status, string(b))
}

// secretPath returns the KV path for a project's secrets.
func secretPath(slug string) string {
	return "/v1/secret/data/kuberlauncher/" + slug
}

// SetSecret creates or updates a key in the project's Vault secret.
func (c *Client) SetSecret(ctx context.Context, slug, key, value string) error {
	// อ่าน secrets ที่มีอยู่ก่อน
	existing, err := c.GetSecrets(ctx, slug)
	if err != nil {
		existing = map[string]string{}
	}
	existing[key] = value

	body := map[string]any{"data": existing}
	b, status, err := c.do(ctx, http.MethodPost, secretPath(slug), body)
	if err != nil {
		return fmt.Errorf("vault set: %w", err)
	}
	if status != http.StatusOK && status != http.StatusNoContent {
		return fmt.Errorf("vault set %d: %s", status, string(b))
	}
	return nil
}

// GetSecrets returns all key-value pairs for a project (values included).
func (c *Client) GetSecrets(ctx context.Context, slug string) (map[string]string, error) {
	b, status, err := c.do(ctx, http.MethodGet, secretPath(slug), nil)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return map[string]string{}, nil
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("vault get %d: %s", status, string(b))
	}

	var result struct {
		Data struct {
			Data map[string]string `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}
	return result.Data.Data, nil
}

// DeleteSecret removes a single key from the project's secret.
func (c *Client) DeleteSecret(ctx context.Context, slug, key string) error {
	existing, err := c.GetSecrets(ctx, slug)
	if err != nil {
		return err
	}
	delete(existing, key)

	body := map[string]any{"data": existing}
	b, status, err := c.do(ctx, http.MethodPost, secretPath(slug), body)
	if err != nil {
		return fmt.Errorf("vault delete: %w", err)
	}
	if status != http.StatusOK && status != http.StatusNoContent {
		return fmt.Errorf("vault delete %d: %s", status, string(b))
	}
	return nil
}

// ListSecretKeys returns only the keys (not values) for a project.
func (c *Client) ListSecretKeys(ctx context.Context, slug string) ([]string, error) {
	secrets, err := c.GetSecrets(ctx, slug)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}
	return keys, nil
}
