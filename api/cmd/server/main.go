package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jakkayy/kuberlauncher/api/config"
	"github.com/jakkayy/kuberlauncher/api/internal/argocd"
	"github.com/jakkayy/kuberlauncher/api/internal/auth"
	"github.com/jakkayy/kuberlauncher/api/internal/db"
	"github.com/jakkayy/kuberlauncher/api/internal/generator"
	gh "github.com/jakkayy/kuberlauncher/api/internal/github"
	"github.com/jakkayy/kuberlauncher/api/internal/grafana"
	"github.com/jakkayy/kuberlauncher/api/internal/handler"
	"github.com/jakkayy/kuberlauncher/api/internal/kube"
	"github.com/jakkayy/kuberlauncher/api/internal/model"
	"github.com/jakkayy/kuberlauncher/api/internal/repository"
	"github.com/jakkayy/kuberlauncher/api/internal/service"
	"github.com/jakkayy/kuberlauncher/api/internal/vault"
)

func main() {
	cfg := config.Load()

	database, err := db.Connect(cfg.DB.DSN())
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	var githubClient *gh.Client
	switch {
	case !cfg.GitHub.Enabled():
	case cfg.GitHub.AppID != 0:
		githubClient, err = gh.NewApp(cfg.GitHub.AppID, cfg.GitHub.InstallationID, cfg.GitHub.PrivateKey, cfg.GitHub.Owner)
		if err != nil {
			log.Fatalf("github app: %v", err)
		}
		log.Printf("GitHub integration enabled via GitHub App %d (owner: %s)", cfg.GitHub.AppID, cfg.GitHub.Owner)
	default:
		githubClient = gh.New(cfg.GitHub.Token, cfg.GitHub.Owner, cfg.GitHub.OwnerIsOrg)
		log.Printf("WARNING: GitHub integration uses a personal access token; prefer a GitHub App (owner: %s)", cfg.GitHub.Owner)
	}

	var argocdClient *argocd.Client
	if cfg.ArgoCD.Enabled() {
		argocdClient = argocd.New(cfg.ArgoCD.URL, cfg.ArgoCD.Token, cfg.ArgoCD.Username, cfg.ArgoCD.Password)
		if cfg.ArgoCD.Token == "" {
			log.Printf("WARNING: ArgoCD integration logs in with a password; prefer ARGOCD_TOKEN for a dedicated account")
		}
		log.Printf("ArgoCD integration enabled (%s)", cfg.ArgoCD.URL)
	}

	var grafanaClient *grafana.Client
	if cfg.Grafana.Password != "" {
		grafanaClient = grafana.New(cfg.Grafana.URL, cfg.Grafana.PublicURL, cfg.Grafana.Username, cfg.Grafana.Password)
		log.Printf("Grafana integration enabled (%s)", cfg.Grafana.URL)
	}

	var vaultClient *vault.Client
	switch {
	case !cfg.Vault.Enabled():
	case cfg.Vault.AuthMethod == "kubernetes":
		vaultClient = vault.NewKubernetes(cfg.Vault.URL, vault.KubernetesAuth{
			Mount: cfg.Vault.K8sMount, Role: cfg.Vault.K8sRole, TokenPath: cfg.Vault.K8sTokenPath,
		})
		log.Printf("Vault integration enabled via Kubernetes auth (%s, role %s)", cfg.Vault.URL, cfg.Vault.K8sRole)
	default:
		vaultClient = vault.New(cfg.Vault.URL, cfg.Vault.Token)
		log.Printf("WARNING: Vault integration uses a static token; prefer VAULT_AUTH_METHOD=kubernetes (%s)", cfg.Vault.URL)
	}

	kubeClient, err := kube.New()
	if err != nil {
		log.Printf("Kubernetes integration disabled: %v", err)
		kubeClient = nil
	}

	userRepo := repository.NewUserRepository(database)
	authn, err := auth.New(cfg.Auth.JWTSecret, userRepo)
	if err != nil {
		log.Fatalf("auth: %v", err)
	}

	accessSvc := service.NewAccessService(repository.NewAccessRepository(database), userRepo, cfg.Auth.Admins)
	accessHandler := handler.NewAccessHandler(accessSvc)

	projectRepo := repository.NewProjectRepository(database)
	deployRepo := repository.NewDeploymentRepository(database)
	envRepo := repository.NewEnvironmentRepository(database)
	projectSvc := service.NewProjectService(service.ProjectDeps{
		Projects:     projectRepo,
		Environments: envRepo,
		GitHub:       githubClient,
		ArgoCD:       argocdClient,
		Grafana:      grafanaClient,
		Vault:        vaultClient,
		Generator: generator.Settings{
			VaultServer:    cfg.Vault.ClusterURL,
			VaultAuthMount: cfg.Vault.K8sMount,
		},
		Hooks:           githubHooks(cfg),
		ArgoCDPublicURL: cfg.ArgoCD.PublicURL,
	})
	deploymentSvc := service.NewDeploymentService(deployRepo, projectRepo, envRepo, githubClient, argocdClient)
	secretSvc := service.NewSecretService(projectRepo, vaultClient)
	projectHandler := handler.NewProjectHandler(projectSvc, accessSvc)
	repoHandler := handler.NewRepoHandler(projectSvc)
	argocdHandler := handler.NewArgoCDHandler(projectSvc)
	monitoringHandler := handler.NewMonitoringHandler(projectSvc)
	setupHandler := handler.NewSetupHandler(service.NewSetupService(projectSvc, repository.NewSetupRepository(database)))
	deploymentHandler := handler.NewDeploymentHandler(deploymentSvc, projectSvc)
	secretHandler := handler.NewSecretHandler(secretSvc)
	logsHandler := handler.NewLogsHandler(projectSvc, kubeClient)

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Webhooks authenticate with their own signatures, not user sessions.
	webhookHandler := handler.NewWebhookHandler(deploymentSvc, cfg.Webhooks.GitHubSecret, cfg.Webhooks.ArgoCDToken)
	r.POST("/webhooks/github", webhookHandler.GitHub)
	r.POST("/webhooks/argocd", webhookHandler.ArgoCD)

	viewer := accessHandler.RequireProjectRole(model.RoleViewer)
	maintainer := accessHandler.RequireProjectRole(model.RoleMaintainer)
	owner := accessHandler.RequireProjectRole(model.RoleOwner)

	auditHandler := handler.NewAuditHandler(repository.NewAuditRepository(database))

	v1 := r.Group("/api/v1", authn.Middleware(), auditHandler.Middleware())
	{
		v1.GET("/me", accessHandler.Me)
		v1.GET("/audit", accessHandler.RequireAdmin, auditHandler.ListAll)

		teams := v1.Group("/teams")
		teams.GET("", accessHandler.ListTeams)
		teams.POST("", accessHandler.CreateTeam)
		teams.GET("/:team/members", accessHandler.ListTeamMembers)
		teams.PUT("/:team/members", accessHandler.AddTeamMember)
		teams.DELETE("/:team/members/:login", accessHandler.RemoveTeamMember)

		projects := v1.Group("/projects")
		projects.POST("", projectHandler.Create)
		projects.GET("", projectHandler.List)

		// viewer: read-only access
		projects.GET("/:id", viewer, projectHandler.GetByID)
		projects.GET("/:id/download", viewer, projectHandler.Download)
		projects.GET("/:id/preview", viewer, projectHandler.Preview)
		projects.GET("/:id/environments", viewer, argocdHandler.Environments)
		projects.GET("/:id/deployments", viewer, deploymentHandler.List)
		projects.GET("/:id/deployments/:dep_id", viewer, deploymentHandler.Get)
		projects.GET("/:id/deployments/:dep_id/stream", viewer, deploymentHandler.Stream)
		projects.GET("/:id/secrets", viewer, secretHandler.ListKeys)
		projects.GET("/:id/members", viewer, accessHandler.ListMembers)
		projects.GET("/:id/setup", viewer, setupHandler.Status)
		projects.GET("/:id/setup/stream", viewer, setupHandler.Stream)
		projects.GET("/:id/audit", maintainer, auditHandler.ListForProject)

		// maintainer: day-to-day operations
		projects.POST("/:id/repo", maintainer, repoHandler.Connect)
		projects.POST("/:id/repo/repair", maintainer, repoHandler.Repair)
		projects.POST("/:id/templates/upgrade", maintainer, repoHandler.UpgradeTemplates)
		projects.POST("/:id/argocd", maintainer, argocdHandler.Register)
		projects.POST("/:id/monitoring", maintainer, monitoringHandler.Setup)
		projects.POST("/:id/setup", maintainer, setupHandler.Start)
		projects.POST("/:id/deployments", maintainer, deploymentHandler.Trigger)
		projects.POST("/:id/secrets", maintainer, secretHandler.Set)
		projects.DELETE("/:id/secrets/:key", maintainer, secretHandler.Delete)
		projects.GET("/:id/logs", maintainer, logsHandler.Stream)

		// owner: destructive actions and access management
		projects.DELETE("/:id", owner, projectHandler.Delete)
		projects.POST("/:id/environments/:env/rollback", owner, argocdHandler.Rollback)
		projects.PUT("/:id/members", owner, accessHandler.AddMember)
		projects.DELETE("/:id/members/:member_id", owner, accessHandler.RemoveMember)
	}

	watchCtx, stopWatch := context.WithCancel(context.Background())
	defer stopWatch()
	// With webhooks delivering status changes, polling is only a safety net.
	pollInterval := 10 * time.Second
	if githubHooks(cfg).URL != "" && cfg.Webhooks.ArgoCDToken != "" {
		pollInterval = time.Minute
	}
	go deploymentSvc.Watch(watchCtx, pollInterval)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: r,
	}

	go func() {
		log.Printf("server listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	stopWatch()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown: %v", err)
	}
}

// githubHooks returns where generated repos should deliver workflow_run
// events, or an empty target when webhooks are not configured.
func githubHooks(cfg *config.Config) service.WebhookTarget {
	if cfg.PublicURL == "" || cfg.Webhooks.GitHubSecret == "" {
		return service.WebhookTarget{}
	}
	return service.WebhookTarget{URL: cfg.PublicURL + "/webhooks/github", Secret: cfg.Webhooks.GitHubSecret}
}
