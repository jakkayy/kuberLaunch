package generator

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jakkayy/kuberlauncher/api/internal/model"
)

var update = flag.Bool("update", false, "rewrite golden files in testdata/")

func sampleProject(rt model.Runtime) *model.Project {
	return &model.Project{
		Name:     "Demo Service",
		Slug:     "demo-service",
		Runtime:  rt,
		Database: model.DatabasePostgres,
		Cache:    model.CacheRedis,
	}
}

// TestGenerateGolden renders every runtime and compares each file to
// testdata/<runtime>/<path>. Run `go test ./internal/generator -update`
// after an intentional template change and review the diff.
func TestGenerateGolden(t *testing.T) {
	for _, rt := range []model.Runtime{model.RuntimeGo, model.RuntimeNextJS, model.RuntimeNestJS, model.RuntimeFastAPI} {
		t.Run(string(rt), func(t *testing.T) {
			files, err := Generate(sampleProject(rt), "acme")
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			dir := filepath.Join("testdata", string(rt))
			if *update {
				if err := os.RemoveAll(dir); err != nil {
					t.Fatal(err)
				}
			}
			for _, f := range files {
				golden := filepath.Join(dir, f.Path)
				if *update {
					if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(golden, []byte(f.Content), 0o644); err != nil {
						t.Fatal(err)
					}
					continue
				}
				want, err := os.ReadFile(golden)
				if err != nil {
					t.Fatalf("missing golden file %s (run with -update): %v", golden, err)
				}
				if string(want) != f.Content {
					t.Errorf("%s differs from golden file; run with -update and review the diff", f.Path)
				}
			}
		})
	}
}

func TestGenerateDefaultsOwner(t *testing.T) {
	files, err := Generate(sampleProject(model.RuntimeGo), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Path == "helm/values.yaml" && !contains(f.Content, "ghcr.io/org/demo-service") {
			t.Errorf("values.yaml should fall back to owner %q:\n%s", "org", f.Content)
		}
	}
}

func TestRuntimePort(t *testing.T) {
	cases := map[model.Runtime]int{
		model.RuntimeGo: 8080, model.RuntimeNextJS: 3000, model.RuntimeNestJS: 3000, model.RuntimeFastAPI: 8000,
	}
	for rt, want := range cases {
		if got := runtimePort(rt); got != want {
			t.Errorf("runtimePort(%s) = %d, want %d", rt, got, want)
		}
	}
}

func TestToZipContainsAllFiles(t *testing.T) {
	files := []GeneratedFile{{Path: "a.txt", Content: "a"}, {Path: "dir/b.txt", Content: "b"}}
	b, err := ToZip(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 {
		t.Fatal("empty zip")
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
