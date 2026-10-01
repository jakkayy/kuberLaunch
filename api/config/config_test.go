package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetSecretPrefersFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DB_PASSWORD", "from-env")
	t.Setenv("DB_PASSWORD_FILE", path)
	if got := getSecret("DB_PASSWORD", ""); got != "from-file" {
		t.Fatalf("got %q, want from-file", got)
	}
}

func TestGetSecretFallsBackToEnv(t *testing.T) {
	t.Setenv("DB_PASSWORD", "from-env")
	if got := getSecret("DB_PASSWORD", "default"); got != "from-env" {
		t.Fatalf("got %q", got)
	}
	if got := getSecret("UNSET_SECRET_FOR_TEST", "default"); got != "default" {
		t.Fatalf("got %q", got)
	}
}

func TestDSNIncludesSSLMode(t *testing.T) {
	d := DBConfig{Host: "db", Port: "5432", User: "u", Password: "p", Name: "n", SSLMode: "require"}
	if got, want := d.DSN(), "host=db port=5432 user=u password=p dbname=n sslmode=require"; got != want {
		t.Fatalf("DSN = %q, want %q", got, want)
	}
}
