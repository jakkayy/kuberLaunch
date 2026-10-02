package kube

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	cs kubernetes.Interface
}

// New uses the in-cluster service account when running inside Kubernetes,
// and falls back to the local kubeconfig ($KUBECONFIG or ~/.kube/config).
func New() (*Client, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		rules := clientcmd.NewDefaultClientConfigLoadingRules()
		cfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("load kube config: %w", err)
		}
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{cs: cs}, nil
}

// NewForClientset wraps an existing clientset (used by tests with a fake clientset).
func NewForClientset(cs kubernetes.Interface) *Client {
	return &Client{cs: cs}
}

// ManagedByLabel marks namespaces and secrets kuberLaunch created.
const ManagedByLabel = "app.kubernetes.io/managed-by"

// EnsureNamespace creates namespace if it does not exist.
func (c *Client) EnsureNamespace(ctx context.Context, namespace string) error {
	_, err := c.cs.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace, Labels: map[string]string{ManagedByLabel: "kuberlaunch"}},
	}, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create namespace %s: %w", namespace, err)
	}
	return nil
}

// RegistryCredentials authenticate image pulls from a container registry.
type RegistryCredentials struct {
	Server   string // e.g. ghcr.io
	Username string
	Token    string // read-only (read:packages) token
}

// EnsurePullSecret creates or updates a kubernetes.io/dockerconfigjson
// secret named name in namespace (creating the namespace if needed).
func (c *Client) EnsurePullSecret(ctx context.Context, namespace, name string, creds RegistryCredentials) error {
	if err := c.EnsureNamespace(ctx, namespace); err != nil {
		return err
	}
	auth := base64.StdEncoding.EncodeToString([]byte(creds.Username + ":" + creds.Token))
	cfg, err := json.Marshal(map[string]any{
		"auths": map[string]any{
			creds.Server: map[string]string{"username": creds.Username, "password": creds.Token, "auth": auth},
		},
	})
	if err != nil {
		return err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: map[string]string{ManagedByLabel: "kuberlaunch"}},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data:       map[string][]byte{corev1.DockerConfigJsonKey: cfg},
	}
	secrets := c.cs.CoreV1().Secrets(namespace)
	if _, err := secrets.Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create pull secret: %w", err)
		}
		if _, err := secrets.Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("update pull secret: %w", err)
		}
	}
	return nil
}

// StreamLogs follows logs of every container in pods matching selector and
// sends each line, prefixed with "[pod/container]", to out. It returns when
// all streams end or ctx is cancelled, and closes out before returning.
func (c *Client) StreamLogs(ctx context.Context, namespace, selector string, tail int64, out chan<- string) error {
	defer close(out)

	pods, err := c.cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return fmt.Errorf("list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found in namespace %s matching %s", namespace, selector)
	}

	var wg sync.WaitGroup
	for _, pod := range pods.Items {
		for _, ctr := range pod.Spec.Containers {
			wg.Add(1)
			go func(pod, container string) {
				defer wg.Done()
				c.followContainer(ctx, namespace, pod, container, tail, out)
			}(pod.Name, ctr.Name)
		}
	}
	wg.Wait()
	return nil
}

func (c *Client) followContainer(ctx context.Context, namespace, pod, container string, tail int64, out chan<- string) {
	prefix := fmt.Sprintf("[%s/%s] ", pod, container)
	stream, err := c.cs.CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{
		Container: container,
		Follow:    true,
		TailLines: &tail,
	}).Stream(ctx)
	if err != nil {
		send(ctx, out, prefix+"error: "+err.Error())
		return
	}
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		if !send(ctx, out, prefix+scanner.Text()) {
			return
		}
	}
}

func send(ctx context.Context, out chan<- string, line string) bool {
	select {
	case out <- line:
		return true
	case <-ctx.Done():
		return false
	}
}
