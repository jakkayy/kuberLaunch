package generator

import (
	"archive/zip"
	"bytes"
	"embed"
	"fmt"
	"path/filepath"
	"text/template"

	"github.com/jakkayy/kuberlauncher/api/internal/model"
)

//go:embed all:templates
var templateFS embed.FS

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
	for _, ts := range templateSets {
		content, err := renderTemplate(ts.templatePath, data)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", ts.templatePath, err)
		}
		files = append(files, GeneratedFile{Path: ts.outputPath, Content: content})
	}
	return files, nil
}

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
		{"templates/common/argocd/appproject.yaml.tmpl", "argocd/appproject.yaml"},
		{"templates/common/argocd/application.yaml.tmpl", "argocd/application.yaml"},
		{"templates/common/github/ci.yml.tmpl", ".github/workflows/ci.yml"},
		{"templates/common/monitoring/servicemonitor.yaml.tmpl", "monitoring/servicemonitor.yaml"},
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
