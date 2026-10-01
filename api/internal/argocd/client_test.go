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

func TestEnsureProjectScopesRepoAndNamespaces(t *testing.T) {
	c, calls := fakeArgo(t)
	if err := c.EnsureProject(context.Background(), "demo", "https://github.com/acme/demo"); err != nil {
		t.Fatal(err)
	}
	proj := (*calls)[0]
	if proj.path != "/api/v1/projects" || proj.body["upsert"] != true {
		t.Fatalf("project call = %+v", proj)
	}
	spec := proj.body["project"].(map[string]any)["spec"].(map[string]any)
	if repos := spec["sourceRepos"].([]any); len(repos) != 1 || repos[0] != "https://github.com/acme/demo" {
		t.Errorf("sourceRepos = %v", repos)
	}
	if dest := spec["destinations"].([]any)[0].(map[string]any); dest["namespace"] != "demo-*" {
		t.Errorf("destination namespace = %v", dest["namespace"])
	}
}

func TestRegisterApp(t *testing.T) {
	for _, auto := range []bool{true, false} {
		c, calls := fakeArgo(t)
		err := c.RegisterApp(context.Background(), AppSpec{
			Name: "demo-prod", Project: "demo", RepoURL: "https://github.com/acme/demo", ReleaseName: "demo",
			ValueFiles: []string{"values.yaml", "values-prod.yaml"}, Namespace: "demo-prod", AutoSync: auto,
		})
		if err != nil {
			t.Fatal(err)
		}
		app := (*calls)[0]
		if app.path != "/api/v1/applications" || app.query != "upsert=true" {
			t.Fatalf("app call = %+v", app)
		}
		spec := app.body["spec"].(map[string]any)
		if spec["project"] != "demo" {
			t.Errorf("project = %v", spec["project"])
		}
		helm := spec["source"].(map[string]any)["helm"].(map[string]any)
		if helm["releaseName"] != "demo" || len(helm["valueFiles"].([]any)) != 2 {
			t.Errorf("helm = %v", helm)
		}
		_, hasAuto := spec["syncPolicy"].(map[string]any)["automated"]
		if hasAuto != auto {
			t.Errorf("AutoSync=%v but automated present=%v", auto, hasAuto)
		}
	}
}
