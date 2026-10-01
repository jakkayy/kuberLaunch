package kube

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestStreamLogsPrefixesEachContainer(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: "demo-dev", Labels: map[string]string{"app": "demo"}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
	})
	out := make(chan string, 10)
	if err := NewForClientset(cs).StreamLogs(context.Background(), "demo-dev", "app=demo", 10, out); err != nil {
		t.Fatalf("StreamLogs: %v", err)
	}
	var got []string
	for l := range out {
		got = append(got, l)
	}
	if len(got) != 1 || got[0] != "[web-1/app] fake logs" {
		t.Fatalf("got %q", got)
	}
}

func TestStreamLogsNoPods(t *testing.T) {
	out := make(chan string, 1)
	err := NewForClientset(fake.NewSimpleClientset()).StreamLogs(context.Background(), "demo-dev", "app=demo", 10, out)
	if err == nil {
		t.Fatal("expected error when no pods match")
	}
	if _, open := <-out; open {
		t.Fatal("out should be closed")
	}
}

func TestEnsurePullSecret(t *testing.T) {
	cs := fake.NewSimpleClientset()
	c := NewForClientset(cs)
	creds := RegistryCredentials{Server: "ghcr.io", Username: "bot", Token: "t1"}
	ctx := context.Background()

	if err := c.EnsurePullSecret(ctx, "demo-dev", "ghcr-secret", creds); err != nil {
		t.Fatal(err)
	}
	if _, err := cs.CoreV1().Namespaces().Get(ctx, "demo-dev", metav1.GetOptions{}); err != nil {
		t.Fatalf("namespace not created: %v", err)
	}

	// Rotating the token updates the existing secret.
	creds.Token = "t2"
	if err := c.EnsurePullSecret(ctx, "demo-dev", "ghcr-secret", creds); err != nil {
		t.Fatal(err)
	}
	sec, err := cs.CoreV1().Secrets("demo-dev").Get(ctx, "ghcr-secret", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if sec.Type != corev1.SecretTypeDockerConfigJson {
		t.Fatalf("type = %s", sec.Type)
	}
	if cfg := string(sec.Data[corev1.DockerConfigJsonKey]); !strings.Contains(cfg, `"password":"t2"`) || !strings.Contains(cfg, `"ghcr.io"`) {
		t.Fatalf("dockerconfigjson = %s", cfg)
	}
}
