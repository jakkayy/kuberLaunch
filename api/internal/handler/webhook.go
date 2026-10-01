package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jakkayy/kuberlauncher/api/internal/argocd"
	gh "github.com/jakkayy/kuberlauncher/api/internal/github"
	"github.com/jakkayy/kuberlauncher/api/internal/service"
)

// maxWebhookBody caps webhook payloads read into memory.
const maxWebhookBody = 5 << 20

type WebhookHandler struct {
	deployments  *service.DeploymentService
	githubSecret []byte
	argocdToken  string
}

func NewWebhookHandler(deployments *service.DeploymentService, githubSecret, argocdToken string) *WebhookHandler {
	return &WebhookHandler{deployments: deployments, githubSecret: []byte(githubSecret), argocdToken: argocdToken}
}

// GitHub receives repository webhooks. Only workflow_run events are used.
// POST /webhooks/github
func (h *WebhookHandler) GitHub(c *gin.Context) {
	if len(h.githubSecret) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "GitHub webhooks not configured"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxWebhookBody))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body"})
		return
	}
	if !validGitHubSignature(h.githubSecret, body, c.GetHeader("X-Hub-Signature-256")) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}
	if c.GetHeader("X-GitHub-Event") != "workflow_run" {
		c.Status(http.StatusNoContent)
		return
	}

	var payload struct {
		WorkflowRun struct {
			ID         int64     `json:"id"`
			Status     string    `json:"status"`
			Conclusion string    `json:"conclusion"`
			HeadSHA    string    `json:"head_sha"`
			HeadBranch string    `json:"head_branch"`
			Event      string    `json:"event"`
			Path       string    `json:"path"`
			CreatedAt  time.Time `json:"created_at"`
		} `json:"workflow_run"`
		Repository struct {
			Name string `json:"name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	run := payload.WorkflowRun
	if !strings.HasSuffix(run.Path, "/ci.yml") {
		c.Status(http.StatusNoContent)
		return
	}
	err = h.deployments.HandleWorkflowRun(c.Request.Context(), service.WorkflowRunEvent{
		Repo:      payload.Repository.Name,
		Branch:    run.HeadBranch,
		Event:     run.Event,
		CreatedAt: run.CreatedAt,
		Run:       gh.WorkflowRun{ID: run.ID, Status: run.Status, Conclusion: run.Conclusion, HeadSHA: run.HeadSHA},
	})
	if err != nil {
		log.Printf("github webhook: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func validGitHubSignature(secret, body []byte, header string) bool {
	sig, ok := strings.CutPrefix(header, "sha256=")
	if !ok {
		return false
	}
	got, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hmac.Equal(got, mac.Sum(nil))
}

// ArgoCD receives argocd-notifications webhooks
// (k8s/platform/argocd-notifications.yaml).
// POST /webhooks/argocd
func (h *WebhookHandler) ArgoCD(c *gin.Context) {
	if h.argocdToken == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "ArgoCD webhooks not configured"})
		return
	}
	token, _ := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(token), []byte(h.argocdToken)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	var payload struct {
		App      string   `json:"app"`
		Health   string   `json:"health"`
		Sync     string   `json:"sync"`
		Revision string   `json:"revision"`
		Images   []string `json:"images"`
	}
	if err := json.NewDecoder(io.LimitReader(c.Request.Body, maxWebhookBody)).Decode(&payload); err != nil || payload.App == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	err := h.deployments.HandleAppStatus(c.Request.Context(), payload.App, &argocd.AppStatus{
		Health: payload.Health, Sync: payload.Sync, Revision: payload.Revision, Images: payload.Images,
	})
	if err != nil {
		log.Printf("argocd webhook: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
