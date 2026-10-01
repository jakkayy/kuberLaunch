package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jakkayy/kuberlauncher/api/internal/argocd"
	gh "github.com/jakkayy/kuberlauncher/api/internal/github"
	"github.com/jakkayy/kuberlauncher/api/internal/model"
)

type fakeDeployStore struct {
	byID map[string]*model.Deployment
}

func newFakeDeployStore(ds ...*model.Deployment) *fakeDeployStore {
	s := &fakeDeployStore{byID: map[string]*model.Deployment{}}
	for _, d := range ds {
		s.byID[d.ID] = d
	}
	return s
}

func (s *fakeDeployStore) Create(_ context.Context, d *model.Deployment) error {
	d.ID = "dep-new"
	d.CreatedAt = time.Now()
	s.byID[d.ID] = d
	return nil
}
func (s *fakeDeployStore) FindByID(_ context.Context, id string) (*model.Deployment, error) {
	d, ok := s.byID[id]
	if !ok {
		return nil, nil
	}
	cp := *d
	return &cp, nil
}
func (s *fakeDeployStore) FindByProject(context.Context, string) ([]model.Deployment, error) {
	return nil, nil
}
func (s *fakeDeployStore) FindActive(context.Context) ([]model.Deployment, error) {
	var out []model.Deployment
	for _, d := range s.byID {
		if !d.Status.IsTerminal() {
			out = append(out, *d)
		}
	}
	return out, nil
}
func (s *fakeDeployStore) UpdateStatus(_ context.Context, id string, st model.DeploymentStatus) error {
	s.byID[id].Status = st
	return nil
}
func (s *fakeDeployStore) SetWorkflowRunID(_ context.Context, id string, runID int64) error {
	s.byID[id].WorkflowRunID = &runID
	return nil
}
func (s *fakeDeployStore) SetImage(_ context.Context, id, image string) error {
	s.byID[id].Image = image
	return nil
}

type fakeProjects map[string]*model.Project

func (f fakeProjects) FindByID(_ context.Context, id string) (*model.Project, error) {
	return f[id], nil
}

type fakeGitHub struct {
	dispatched *gh.WorkflowRun // returned by FindDispatchedRun
	byID       map[int64]*gh.WorkflowRun
	triggerErr error
	triggered  []string
	since      time.Time
}

func (f *fakeGitHub) Owner() string { return "Acme" }
func (f *fakeGitHub) TriggerWorkflow(_ context.Context, slug, branch string) error {
	f.triggered = append(f.triggered, slug+"@"+branch)
	return f.triggerErr
}
func (f *fakeGitHub) FindDispatchedRun(_ context.Context, _, _ string, since time.Time) (*gh.WorkflowRun, error) {
	f.since = since
	return f.dispatched, nil
}
func (f *fakeGitHub) GetWorkflowRun(_ context.Context, _ string, id int64) (*gh.WorkflowRun, error) {
	if r, ok := f.byID[id]; ok {
		return r, nil
	}
	return nil, errors.New("run not found")
}
func (f *fakeGitHub) GetFileContent(context.Context, string, string) (string, error) {
	return "on:\n  workflow_dispatch:\n", nil
}
func (f *fakeGitHub) UpdateFile(context.Context, string, string, string, string) error { return nil }

type fakeArgo struct {
	status *argocd.AppStatus
	err    error
}

func (f *fakeArgo) GetAppStatus(context.Context, string) (*argocd.AppStatus, error) {
	return f.status, f.err
}

var demoProject = &model.Project{ID: "p1", Slug: "demo", RepoURL: "https://github.com/Acme/demo", ArgocdApp: "demo"}

func newTestService(store *fakeDeployStore, github *fakeGitHub, argo *fakeArgo) *DeploymentService {
	s := &DeploymentService{deployRepo: store, projectRepo: fakeProjects{"p1": demoProject}}
	if github != nil {
		s.github = github
	}
	if argo != nil {
		s.argocd = argo
	}
	return s
}

func deployment(status model.DeploymentStatus) *model.Deployment {
	return &model.Deployment{ID: "d1", ProjectID: "p1", Branch: "main", Status: status, CreatedAt: time.Now()}
}

func TestPollStatusBuilding(t *testing.T) {
	cases := []struct {
		name       string
		run        *gh.WorkflowRun
		wantStatus model.DeploymentStatus
		wantImage  string
	}{
		{"run not created yet", nil, model.DeploymentStatusBuilding, ""},
		{"run in progress", &gh.WorkflowRun{ID: 7, Status: "in_progress"}, model.DeploymentStatusBuilding, ""},
		{"run succeeded", &gh.WorkflowRun{ID: 7, Status: "completed", Conclusion: "success", HeadSHA: "abcdef1234"},
			model.DeploymentStatusDeploying, "ghcr.io/acme/demo:sha-abcdef1"},
		{"run failed", &gh.WorkflowRun{ID: 7, Status: "completed", Conclusion: "failure"}, model.DeploymentStatusFailed, ""},
		{"run cancelled", &gh.WorkflowRun{ID: 7, Status: "completed", Conclusion: "cancelled"}, model.DeploymentStatusFailed, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeDeployStore(deployment(model.DeploymentStatusBuilding))
			s := newTestService(store, &fakeGitHub{dispatched: tc.run}, nil)

			got, err := s.PollStatus(context.Background(), "d1")
			if err != nil {
				t.Fatalf("PollStatus: %v", err)
			}
			if got != tc.wantStatus || store.byID["d1"].Status != tc.wantStatus {
				t.Errorf("status = %s (stored %s), want %s", got, store.byID["d1"].Status, tc.wantStatus)
			}
			if store.byID["d1"].Image != tc.wantImage {
				t.Errorf("image = %q, want %q", store.byID["d1"].Image, tc.wantImage)
			}
			if tc.run != nil && (store.byID["d1"].WorkflowRunID == nil || *store.byID["d1"].WorkflowRunID != tc.run.ID) {
				t.Errorf("workflow run ID not recorded")
			}
		})
	}
}

func TestPollStatusUsesStoredRunID(t *testing.T) {
	d := deployment(model.DeploymentStatusBuilding)
	runID := int64(42)
	d.WorkflowRunID = &runID
	store := newFakeDeployStore(d)
	github := &fakeGitHub{
		// A newer unrelated run must be ignored once the run ID is known.
		dispatched: &gh.WorkflowRun{ID: 99, Conclusion: "failure"},
		byID:       map[int64]*gh.WorkflowRun{42: {ID: 42, Conclusion: "success", HeadSHA: "1234567"}},
	}
	got, _ := newTestService(store, github, nil).PollStatus(context.Background(), "d1")
	if got != model.DeploymentStatusDeploying {
		t.Fatalf("status = %s, want deploying", got)
	}
}

func TestPollStatusLooksUpRunsCreatedAfterDeployment(t *testing.T) {
	d := deployment(model.DeploymentStatusBuilding)
	github := &fakeGitHub{}
	_, _ = newTestService(newFakeDeployStore(d), github, nil).PollStatus(context.Background(), "d1")
	if want := d.CreatedAt.Add(-runLookupSkew); !github.since.Equal(want) {
		t.Errorf("since = %v, want %v", github.since, want)
	}
}

func TestPollStatusDeploying(t *testing.T) {
	const image = "ghcr.io/acme/demo:sha-abcdef1"
	cases := []struct {
		name   string
		status *argocd.AppStatus
		err    error
		want   model.DeploymentStatus
	}{
		{"healthy with new image", &argocd.AppStatus{Health: "Healthy", Sync: "Synced", Images: []string{"ghcr.io/Acme/demo:sha-abcdef1"}}, nil, model.DeploymentStatusSuccess},
		{"healthy with old image", &argocd.AppStatus{Health: "Healthy", Sync: "Synced", Images: []string{"ghcr.io/acme/demo:sha-0000000"}}, nil, model.DeploymentStatusDeploying},
		{"healthy, no image info", &argocd.AppStatus{Health: "Healthy", Sync: "Synced"}, nil, model.DeploymentStatusSuccess},
		{"out of sync", &argocd.AppStatus{Health: "Healthy", Sync: "OutOfSync", Images: []string{image}}, nil, model.DeploymentStatusDeploying},
		{"progressing", &argocd.AppStatus{Health: "Progressing", Sync: "Synced"}, nil, model.DeploymentStatusDeploying},
		{"degraded", &argocd.AppStatus{Health: "Degraded", Sync: "Synced"}, nil, model.DeploymentStatusFailed},
		{"argocd unreachable", nil, errors.New("timeout"), model.DeploymentStatusDeploying},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := deployment(model.DeploymentStatusDeploying)
			d.Image = image
			store := newFakeDeployStore(d)
			got, _ := newTestService(store, &fakeGitHub{}, &fakeArgo{status: tc.status, err: tc.err}).PollStatus(context.Background(), "d1")
			if got != tc.want {
				t.Errorf("status = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestPollStatusTerminalIsUnchanged(t *testing.T) {
	for _, st := range []model.DeploymentStatus{model.DeploymentStatusSuccess, model.DeploymentStatusFailed} {
		got, err := newTestService(newFakeDeployStore(deployment(st)), &fakeGitHub{}, nil).PollStatus(context.Background(), "d1")
		if err != nil || got != st {
			t.Errorf("PollStatus(%s) = %s, %v", st, got, err)
		}
	}
}

func TestPollActiveTimesOutStaleDeployments(t *testing.T) {
	d := deployment(model.DeploymentStatusBuilding)
	d.CreatedAt = time.Now().Add(-DeploymentTimeout - time.Minute)
	store := newFakeDeployStore(d)
	newTestService(store, &fakeGitHub{}, nil).pollActive(context.Background())
	if store.byID["d1"].Status != model.DeploymentStatusFailed {
		t.Fatalf("status = %s, want failed", store.byID["d1"].Status)
	}
}

func TestTrigger(t *testing.T) {
	t.Run("rejects non-main branch", func(t *testing.T) {
		github := &fakeGitHub{}
		_, err := newTestService(newFakeDeployStore(), github, nil).Trigger(context.Background(), "p1", "feature/x")
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("err = %v, want ErrInvalid", err)
		}
		if len(github.triggered) != 0 {
			t.Fatal("workflow must not be dispatched")
		}
	})
	t.Run("dispatches main", func(t *testing.T) {
		store := newFakeDeployStore()
		github := &fakeGitHub{}
		d, err := newTestService(store, github, nil).Trigger(context.Background(), "p1", "main")
		if err != nil {
			t.Fatal(err)
		}
		if d.Status != model.DeploymentStatusBuilding || len(github.triggered) != 1 {
			t.Fatalf("status %s, triggered %v", d.Status, github.triggered)
		}
	})
	t.Run("marks failed when dispatch fails", func(t *testing.T) {
		store := newFakeDeployStore()
		_, err := newTestService(store, &fakeGitHub{triggerErr: errors.New("boom")}, nil).Trigger(context.Background(), "p1", "main")
		if err == nil || store.byID["dep-new"].Status != model.DeploymentStatusFailed {
			t.Fatalf("err %v, status %s", err, store.byID["dep-new"].Status)
		}
	})
	t.Run("requires GitHub", func(t *testing.T) {
		if _, err := newTestService(newFakeDeployStore(), nil, nil).Trigger(context.Background(), "p1", "main"); err == nil {
			t.Fatal("expected error without GitHub client")
		}
	})
}

func TestPollStatusFailsWhenProjectDeleted(t *testing.T) {
	d := deployment(model.DeploymentStatusBuilding)
	d.ProjectID = "gone"
	store := newFakeDeployStore(d)
	got, err := newTestService(store, &fakeGitHub{}, nil).PollStatus(context.Background(), "d1")
	if err != nil || got != model.DeploymentStatusFailed || store.byID["d1"].Status != model.DeploymentStatusFailed {
		t.Fatalf("status = %s (stored %s), err %v", got, store.byID["d1"].Status, err)
	}
}
