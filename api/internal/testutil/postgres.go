//go:build integration

// Package testutil starts disposable dependencies for integration tests.
// Run with: go test -tags integration ./...
package testutil

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/jakkayy/kuberlauncher/api/internal/db"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Postgres starts a PostgreSQL 16 container, applies all migrations and
// returns a connection. The container is removed when the test ends.
func Postgres(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("kuberlauncher"),
		postgres.WithUsername("kuberlauncher"),
		postgres.WithPassword("kuberlauncher"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(ctr) })

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return conn
}
