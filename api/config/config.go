package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port   string
	Env    string
	DB     DBConfig
	GitHub GitHubConfig
	ArgoCD ArgoCDConfig
}

type ArgoCDConfig struct {
	URL      string
	Username string
	Password string
}

type GitHubConfig struct {
	Token string
	Owner string
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
		Port: getEnv("PORT", "8080"),
		Env:  getEnv("ENV", "development"),
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "kuberlauncher"),
			Password: getEnv("DB_PASSWORD", "kuberlauncher"),
			Name:     getEnv("DB_NAME", "kuberlauncher"),
		},
		GitHub: GitHubConfig{
			Token: getEnv("GITHUB_TOKEN", ""),
			Owner: getEnv("GITHUB_OWNER", ""),
		},
		ArgoCD: ArgoCDConfig{
			URL:      getEnv("ARGOCD_URL", "http://argocd.localhost:8090"),
			Username: getEnv("ARGOCD_USERNAME", "admin"),
			Password: getEnv("ARGOCD_PASSWORD", ""),
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
