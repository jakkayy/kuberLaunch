package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port string
	Env  string
	// PublicURL is the API's externally reachable base URL, used to register
	// webhooks (e.g. https://kuberlaunch.example.com). Webhooks are off when empty.
	PublicURL string
	Webhooks  WebhookConfig
	Auth      AuthConfig
	DB        DBConfig
	GitHub    GitHubConfig
	ArgoCD    ArgoCDConfig
	Grafana   GrafanaConfig
	Vault     VaultConfig
}

type WebhookConfig struct {
	GitHubSecret string // HMAC secret for X-Hub-Signature-256
	ArgoCDToken  string // bearer token sent by argocd-notifications
}

type AuthConfig struct {
	// JWTSecret verifies session tokens signed by the web app (shared secret).
	JWTSecret string
	// Admins are GitHub logins with owner access to every project and team.
	Admins []string
}

type ArgoCDConfig struct {
	URL       string
	PublicURL string // ArgoCD UI address for links; defaults to URL
	Token     string // API token of a dedicated ArgoCD account (preferred)
	Username  string
	Password  string
}

func (a ArgoCDConfig) Enabled() bool { return a.Token != "" || a.Password != "" }

type GrafanaConfig struct {
	URL       string
	PublicURL string
	Username  string
	Password  string
}

type VaultConfig struct {
	URL string
	// ClusterURL is Vault's address as seen by pods (External Secrets).
	ClusterURL string
	// AuthMethod is "token" (local dev) or "kubernetes".
	AuthMethod string
	Token      string
	// Kubernetes auth settings for the API's own service account.
	K8sMount     string
	K8sRole      string
	K8sTokenPath string
}

func (v VaultConfig) Enabled() bool {
	return v.AuthMethod == "kubernetes" || v.Token != ""
}

type GitHubConfig struct {
	Owner string
	// OwnerIsOrg creates repos under the Owner organization (PAT mode).
	OwnerIsOrg bool

	// Personal access token mode.
	Token string

	// GitHub App mode (preferred). Used when AppID is set.
	AppID          int64
	InstallationID int64
	PrivateKey     []byte
}

func (g GitHubConfig) Enabled() bool {
	return g.Owner != "" && (g.Token != "" || g.AppID != 0)
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

func (d DBConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		d.Host, d.Port, d.User, d.Password, d.Name)
}

func Load() *Config {
	_ = godotenv.Load()

	return &Config{
		Port:      getEnv("PORT", "8080"),
		Env:       getEnv("ENV", "development"),
		PublicURL: strings.TrimRight(getEnv("PLATFORM_PUBLIC_URL", ""), "/"),
		Webhooks: WebhookConfig{
			GitHubSecret: getSecret("GITHUB_WEBHOOK_SECRET", ""),
			ArgoCDToken:  getSecret("ARGOCD_WEBHOOK_TOKEN", ""),
		},
		Auth: AuthConfig{
			JWTSecret: getSecret("AUTH_JWT_SECRET", ""),
			Admins:    strings.Split(getEnv("PLATFORM_ADMINS", ""), ","),
		},
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "kuberlauncher"),
			Password: getSecret("DB_PASSWORD", "kuberlauncher"),
			Name:     getEnv("DB_NAME", "kuberlauncher"),
		},
		GitHub: GitHubConfig{
			Owner:          getEnv("GITHUB_OWNER", ""),
			OwnerIsOrg:     getEnv("GITHUB_OWNER_IS_ORG", "false") == "true",
			Token:          getSecret("GITHUB_TOKEN", ""),
			AppID:          getInt("GITHUB_APP_ID"),
			InstallationID: getInt("GITHUB_APP_INSTALLATION_ID"),
			PrivateKey:     getFileOrEnv("GITHUB_APP_PRIVATE_KEY_PATH", "GITHUB_APP_PRIVATE_KEY"),
		},
		ArgoCD: ArgoCDConfig{
			URL:       getEnv("ARGOCD_URL", "http://argocd.localhost:8090"),
			Token:     getSecret("ARGOCD_TOKEN", ""),
			PublicURL: getEnv("ARGOCD_PUBLIC_URL", getEnv("ARGOCD_URL", "http://argocd.localhost:8090")),
			Username:  getEnv("ARGOCD_USERNAME", "admin"),
			Password:  getSecret("ARGOCD_PASSWORD", ""),
		},
		Grafana: GrafanaConfig{
			URL:       getEnv("GRAFANA_URL", "http://grafana.localhost:8090"),
			PublicURL: getEnv("GRAFANA_PUBLIC_URL", ""),
			Username:  getEnv("GRAFANA_USERNAME", "admin"),
			Password:  getSecret("GRAFANA_PASSWORD", "admin"),
		},
		Vault: VaultConfig{
			URL:          getEnv("VAULT_URL", "http://vault.localhost:8090"),
			ClusterURL:   getEnv("VAULT_CLUSTER_URL", "http://vault.vault.svc.cluster.local:8200"),
			AuthMethod:   getEnv("VAULT_AUTH_METHOD", "token"),
			Token:        getSecret("VAULT_TOKEN", ""),
			K8sMount:     getEnv("VAULT_K8S_MOUNT", "kubernetes"),
			K8sRole:      getEnv("VAULT_K8S_ROLE", "kuberlaunch-api"),
			K8sTokenPath: getEnv("VAULT_K8S_TOKEN_PATH", "/var/run/secrets/kubernetes.io/serviceaccount/token"),
		},
	}
}

func getInt(key string) int64 {
	n, _ := strconv.ParseInt(os.Getenv(key), 10, 64)
	return n
}

// getFileOrEnv reads the file at env pathKey if set, else the value of valueKey.
func getFileOrEnv(pathKey, valueKey string) []byte {
	if path := os.Getenv(pathKey); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("read %s: %v", pathKey, err)
		}
		return b
	}
	return []byte(os.Getenv(valueKey))
}

// getSecret reads a secret from the file named by KEY_FILE when set (e.g. a
// Kubernetes Secret synced from Vault by External Secrets and mounted as a
// volume), otherwise from the KEY environment variable.
func getSecret(key, fallback string) string {
	if path := os.Getenv(key + "_FILE"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("read %s_FILE: %v", key, err)
		}
		return strings.TrimSpace(string(b))
	}
	return getEnv(key, fallback)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
