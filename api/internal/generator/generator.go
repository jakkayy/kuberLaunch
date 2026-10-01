package generator

import (
	"archive/zip"
	"bytes"
	"embed"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/jakkayy/kuberlauncher/api/internal/model"
)

//go:embed all:templates
var templateFS embed.FS

// TemplateVersion identifies the generated file set. Bump it whenever a
// template change should be offered to existing projects as an upgrade PR.
//
//	1: initial templates
//	2: per-project Vault SecretStore, ArgoCD AppProject, CI concurrency
//	3: dev/staging/prod environments, ServiceMonitor inside the Helm chart
const TemplateVersion = 3

// Settings are platform-wide values rendered into every project.
type Settings struct {
	Owner string // GitHub owner of generated repos; "org" when empty

	// VaultServer is the Vault address as seen from inside the cluster.
	VaultServer string
	// VaultAuthMount is the Kubernetes auth mount path in Vault.
	VaultAuthMount string
}

func (s Settings) withDefaults() Settings {
	if s.Owner == "" {
		s.Owner = "org"
	}
	if s.VaultServer == "" {
		s.VaultServer = "http://vault.vault.svc.cluster.local:8200"
	}
	if s.VaultAuthMount == "" {
		s.VaultAuthMount = "kubernetes"
	}
	return s
}

type TemplateData struct {
	Name     string
	Slug     string
	Runtime  string
	Database string
	Cache    string
	Port     int
	Owner    string
	Image    string

	VaultServer    string
	VaultAuthMount string
	VaultRole      string
	VaultSA        string

	// Set only while rendering per-environment templates.
	Env          string
	EnvNamespace string
	EnvApp       string
	AutoSync     bool
	Replicas     int
	IngressHost  string
}

// forEnv returns d with the per-environment fields filled for env.
func (d TemplateData) forEnv(env model.EnvName) TemplateData {
	d.Env = string(env)
	d.EnvNamespace = model.Namespace(d.Slug, env)
	d.EnvApp = model.AppName(d.Slug, env)
	d.AutoSync = env.AutoSync()
	d.Replicas = 1
	d.IngressHost = d.Slug + "-" + string(env) + ".local"
	if env == model.EnvProd {
		d.Replicas = 2
		d.IngressHost = d.Slug + ".local"
	}
	return d
}

type GeneratedFile struct {
	Path    string
	Content string
}

func Generate(p *model.Project, settings Settings) ([]GeneratedFile, error) {
	settings = settings.withDefaults()
	data := TemplateData{
		Name:     p.Name,
		Slug:     p.Slug,
		Runtime:  string(p.Runtime),
		Database: string(p.Database),
		Cache:    string(p.Cache),
		Port:     runtimePort(p.Runtime),
		Owner:    settings.Owner,
		Image:    fmt.Sprintf("ghcr.io/%s/%s:latest", settings.Owner, p.Slug),

		VaultServer:    settings.VaultServer,
		VaultAuthMount: settings.VaultAuthMount,
		// Must match vault.ProjectPolicyName / ProjectServiceAccount.
		VaultRole: "kuberlaunch-" + p.Slug,
		VaultSA:   p.Slug + "-vault",
	}

	templateSets := commonTemplates()
	runtimeSpecific := runtimeTemplates(p.Runtime)
	templateSets = append(templateSets, runtimeSpecific...)

	var files []GeneratedFile
	render := func(templatePath, outputPath string, d TemplateData) error {
		content, err := renderTemplate(templatePath, d)
		if err != nil {
			return fmt.Errorf("render %s: %w", templatePath, err)
		}
		files = append(files, GeneratedFile{Path: outputPath, Content: content})
		return nil
	}
	for _, ts := range templateSets {
		if err := render(ts.templatePath, ts.outputPath, data); err != nil {
			return nil, err
		}
	}
	for _, ts := range envTemplates() {
		for _, env := range model.Environments {
			if err := render(ts.templatePath, strings.ReplaceAll(ts.outputPath, "{env}", string(env)), data.forEnv(env)); err != nil {
				return nil, err
			}
		}
	}
	return files, nil
}

// ValuesFile is the Helm values file holding env's image tag.
func ValuesFile(env model.EnvName) string { return "helm/values-" + string(env) + ".yaml" }

func ToZip(files []GeneratedFile) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range files {
		fw, err := w.Create(f.Path)
		if err != nil {
			return nil, err
		}
		if _, err := fw.Write([]byte(f.Content)); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type templateSpec struct {
	templatePath string
	outputPath   string
}

func commonTemplates() []templateSpec {
	return []templateSpec{
		{"templates/common/helm/Chart.yaml.tmpl", "helm/Chart.yaml"},
		{"templates/common/helm/values.yaml.tmpl", "helm/values.yaml"},
		{"templates/common/helm/templates/deployment.yaml.tmpl", "helm/templates/deployment.yaml"},
		{"templates/common/helm/templates/externalsecret.yaml.tmpl", "helm/templates/externalsecret.yaml"},
		{"templates/common/helm/templates/secretstore.yaml.tmpl", "helm/templates/secretstore.yaml"},
		{"templates/common/helm/templates/service.yaml.tmpl", "helm/templates/service.yaml"},
		{"templates/common/helm/templates/ingress.yaml.tmpl", "helm/templates/ingress.yaml"},
		{"templates/common/helm/templates/servicemonitor.yaml.tmpl", "helm/templates/servicemonitor.yaml"},
		{"templates/common/argocd/appproject.yaml.tmpl", "argocd/appproject.yaml"},
		{"templates/common/github/ci.yml.tmpl", ".github/workflows/ci.yml"},
	}
}

// envTemplates are rendered once per environment; {env} in the output path
// is replaced with the environment name.
func envTemplates() []templateSpec {
	return []templateSpec{
		{"templates/common/helm/values-env.yaml.tmpl", "helm/values-{env}.yaml"},
		{"templates/common/argocd/application.yaml.tmpl", "argocd/application-{env}.yaml"},
	}
}

func runtimeTemplates(rt model.Runtime) []templateSpec {
	return []templateSpec{
		{fmt.Sprintf("templates/%s/Dockerfile.tmpl", rt), "Dockerfile"},
		{fmt.Sprintf("templates/%s/.dockerignore.tmpl", rt), ".dockerignore"},
	}
}

func renderTemplate(path string, data TemplateData) (string, error) {
	content, err := templateFS.ReadFile(path)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New(filepath.Base(path)).Parse(string(content))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func runtimePort(rt model.Runtime) int {
	switch rt {
	case model.RuntimeNextJS:
		return 3000
	case model.RuntimeNestJS:
		return 3000
	case model.RuntimeGo:
		return 8080
	case model.RuntimeFastAPI:
		return 8000
	default:
		return 3000
	}
}
