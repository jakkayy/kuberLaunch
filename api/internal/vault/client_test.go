package vault

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type fakeVault struct {
	mu     sync.Mutex
	logins int
	writes map[string]map[string]any
}

func (f *fakeVault) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path == "/v1/auth/kubernetes/login" {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["jwt"] != "sa-jwt" || body["role"] != "kuberlaunch-api" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		f.logins++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"auth": map[string]any{"client_token": "vault-token", "lease_duration": 3600},
		})
		return
	}
	if r.Header.Get("X-Vault-Token") != "vault-token" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.writes[r.Method+" "+r.URL.Path] = body
	w.WriteHeader(http.StatusNoContent)
}

func newKubeClient(t *testing.T) (*Client, *fakeVault) {
	t.Helper()
	fv := &fakeVault{writes: map[string]map[string]any{}}
	srv := httptest.NewServer(fv)
	t.Cleanup(srv.Close)
	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte("sa-jwt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return NewKubernetes(srv.URL, KubernetesAuth{Mount: "kubernetes", Role: "kuberlaunch-api", TokenPath: tokenPath}), fv
}

func TestEnsureProjectAccess(t *testing.T) {
	c, fv := newKubeClient(t)
	if err := c.EnsureProjectAccess(context.Background(), "kubernetes", "demo", []string{"demo-dev"}); err != nil {
		t.Fatal(err)
	}

	policy := fv.writes["PUT /v1/sys/policies/acl/kuberlaunch-demo"]
	if p, _ := policy["policy"].(string); !strings.Contains(p, `"secret/data/kuberlauncher/demo"`) || !strings.Contains(p, `"read"`) {
		t.Errorf("policy = %v", policy)
	}
	role := fv.writes["POST /v1/auth/kubernetes/role/kuberlaunch-demo"]
	if role == nil {
		t.Fatalf("role not written; writes = %v", fv.writes)
	}
	if sa := role["bound_service_account_names"].([]any); sa[0] != "demo-vault" {
		t.Errorf("service account = %v", sa)
	}
	if ns := role["bound_service_account_namespaces"].([]any); len(ns) != 1 || ns[0] != "demo-dev" {
		t.Errorf("namespaces = %v", ns)
	}
	if fv.logins != 1 {
		t.Errorf("logins = %d, want 1 (token should be cached)", fv.logins)
	}
}

func TestDeleteProjectAccess(t *testing.T) {
	c, fv := newKubeClient(t)
	if err := c.DeleteProjectAccess(context.Background(), "kubernetes", "demo"); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"DELETE /v1/auth/kubernetes/role/kuberlaunch-demo", "DELETE /v1/sys/policies/acl/kuberlaunch-demo"} {
		if _, ok := fv.writes[k]; !ok {
			t.Errorf("missing %s", k)
		}
	}
}

func TestStaticTokenRejected(t *testing.T) {
	fv := &fakeVault{writes: map[string]map[string]any{}}
	srv := httptest.NewServer(fv)
	defer srv.Close()
	err := New(srv.URL, "wrong").EnsureProjectAccess(context.Background(), "kubernetes", "demo", nil)
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err = %v, want 403", err)
	}
}
