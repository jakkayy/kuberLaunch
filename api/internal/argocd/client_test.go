package argocd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type recorded struct {
	method, path, query string
	body                map[string]any
}

func fakeArgo(t *testing.T) (*Client, *[]recorded) {
	t.Helper()
	var calls []recorded
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		calls = append(calls, recorded{r.Method, r.URL.Path, r.URL.RawQuery, body})
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, "tok", "", ""), &calls
}

func TestRegisterAppCreatesScopedProject(t *testing.T) {
	c, calls := fakeArgo(t)
	name, err := c.RegisterApp(context.Background(), "demo", "https://github.com/acme/demo")
	if err != nil || name != "demo" {
		t.Fatalf("RegisterApp = %q, %v", name, err)
	}
	if len(*calls) != 2 {
		t.Fatalf("calls = %+v", *calls)
	}

	proj := (*calls)[0]
	if proj.path != "/api/v1/projects" || proj.body["upsert"] != true {
		t.Fatalf("project call = %+v", proj)
	}
	spec := proj.body["project"].(map[string]any)["spec"].(map[string]any)
	if repos := spec["sourceRepos"].([]any); len(repos) != 1 || repos[0] != "https://github.com/acme/demo" {
		t.Errorf("sourceRepos = %v", repos)
	}
	dest := spec["destinations"].([]any)[0].(map[string]any)
	if dest["namespace"] != "demo-*" {
		t.Errorf("destination namespace = %v", dest["namespace"])
	}

	app := (*calls)[1]
	if app.path != "/api/v1/applications" || app.query != "upsert=true" {
		t.Fatalf("app call = %+v", app)
	}
	if p := app.body["spec"].(map[string]any)["project"]; p != "demo" {
		t.Errorf("app project = %v, want demo", p)
	}
}
