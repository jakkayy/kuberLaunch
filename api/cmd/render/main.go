// Command render writes the files kuberLaunch generates for a sample project
// to a directory, so CI can lint the output (helm lint, actionlint, ...).
//
//	go run ./cmd/render -runtime go -out /tmp/demo
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/jakkayy/kuberlauncher/api/internal/generator"
	"github.com/jakkayy/kuberlauncher/api/internal/model"
)

func main() {
	runtime := flag.String("runtime", "go", "nextjs | nestjs | go | fastapi")
	database := flag.String("database", "postgres", "postgres | mysql | none")
	cache := flag.String("cache", "redis", "redis | none")
	slug := flag.String("slug", "demo-service", "project slug")
	owner := flag.String("owner", "acme", "GitHub owner")
	out := flag.String("out", "", "output directory (required)")
	flag.Parse()
	if *out == "" {
		flag.Usage()
		os.Exit(2)
	}

	p := &model.Project{
		Name:     *slug,
		Slug:     *slug,
		Runtime:  model.Runtime(*runtime),
		Database: model.Database(*database),
		Cache:    model.Cache(*cache),
	}
	files, err := generator.Generate(p, generator.Settings{Owner: *owner})
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		path := filepath.Join(*out, f.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(f.Content), 0o644); err != nil {
			log.Fatal(err)
		}
	}
	log.Printf("wrote %d files to %s", len(files), *out)
}
