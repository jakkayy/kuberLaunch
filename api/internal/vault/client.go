package vault

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
	baseURL string
	token   string
	http    *http.Client
}

func New(url, token string) *Client {
	return &Client{
		baseURL: url,
		token:   token,
		http:    &http.Client{Timeout: 15 * time.Second},
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
	req.Header.Set("X-Vault-Token", c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Host = "vault.localhost"

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return b, resp.StatusCode, nil
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
