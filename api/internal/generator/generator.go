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

type TemplateData struct {
	Name     string
	Slug     string
	Runtime  string
	Database string
	Cache    string
	Port     int
	Image    string
}

type GeneratedFile struct {
	Path    string
	Content string
}

func Generate(p *model.Project) ([]GeneratedFile, error) {
	data := TemplateData{
		Name:     p.Name,
		Slug:     p.Slug,
		Runtime:  string(p.Runtime),
		Database: string(p.Database),
		Cache:    string(p.Cache),
		Port:     runtimePort(p.Runtime),
		Image:    fmt.Sprintf("ghcr.io/org/%s:latest", p.Slug),
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
		{"templates/common/helm/templates/service.yaml.tmpl", "helm/templates/service.yaml"},
		{"templates/common/helm/templates/ingress.yaml.tmpl", "helm/templates/ingress.yaml"},
		{"templates/common/argocd/application.yaml.tmpl", "argocd/application.yaml"},
		{"templates/common/github/ci.yml.tmpl", ".github/workflows/ci.yml"},
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
