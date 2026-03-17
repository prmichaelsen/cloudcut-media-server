# Pattern: Integration Testing

**Namespace**: core-sdk-go
**Category**: Testing
**Created**: 2026-03-17
**Status**: Active

---

## Overview

Integration testing in Go validates that multiple components work together correctly against real dependencies (databases, message queues, external services). This pattern uses build tags to separate integration tests from unit tests, `TestMain` for setup/teardown, and testcontainers-go for managing real service instances.

## Problem

Unit tests with mocks can miss real integration issues: SQL syntax errors, network timeouts, serialization bugs, and schema mismatches. Without a structured approach, integration tests become flaky, slow to run locally, and difficult to maintain in CI.

## Solution

Use Go build tags (`//go:build integration`) to isolate integration tests. Use `TestMain` for one-time setup and teardown of shared resources. Use testcontainers-go to spin up real databases and services in Docker containers, ensuring tests run against real infrastructure without manual setup.

## Implementation

### Build Tags for Separation

Create integration test files with a build tag so they are excluded from `go test ./...` by default:

```go
//go:build integration

package store_test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/lib/pq"
)

func TestUserStore_Integration(t *testing.T) {
	// This test only runs when: go test -tags=integration ./...
}
```

### Using testing.Short() as an Alternative

For simpler setups, skip long-running tests when `-short` is passed:

```go
package store_test

import "testing"

func TestUserStore_WithRealDB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	// ... run against real database
}
```

### TestMain for Setup and Teardown

`TestMain` runs once per package. Use it to start containers, run migrations, and clean up:

```go
//go:build integration

package store_test

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	_ "github.com/lib/pq"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	ctx := context.Background()

	// Start a PostgreSQL container
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:16-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     "testuser",
				"POSTGRES_PASSWORD": "testpass",
				"POSTGRES_DB":       "testdb",
			},
			WaitingFor: wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		log.Fatalf("failed to start container: %v", err)
	}

	// Get connection details
	host, err := container.Host(ctx)
	if err != nil {
		log.Fatalf("failed to get host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		log.Fatalf("failed to get port: %v", err)
	}

	dsn := fmt.Sprintf("postgres://testuser:testpass@%s:%s/testdb?sslmode=disable", host, port.Port())

	testDB, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	// Run migrations
	if err := runMigrations(testDB); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// Run all tests in this package
	code := m.Run()

	// Cleanup
	testDB.Close()
	if err := container.Terminate(ctx); err != nil {
		log.Printf("failed to terminate container: %v", err)
	}

	os.Exit(code)
}

func runMigrations(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id    SERIAL PRIMARY KEY,
			name  TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	return err
}
```

### Complete Integration Test

```go
//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/example/project/store"
)

func TestUserStore_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	s := store.NewPostgresUserStore(testDB)

	// Clean up after the test
	t.Cleanup(func() {
		_, _ = testDB.ExecContext(ctx, "DELETE FROM users WHERE email = $1", "integration@test.com")
	})

	// Create
	user, err := s.Create(ctx, store.CreateUserInput{
		Name:  "Integration Test",
		Email: "integration@test.com",
	})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if user.ID == 0 {
		t.Fatal("Create() returned user with zero ID")
	}

	// Get
	got, err := s.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID(%d) error: %v", user.ID, err)
	}
	if got.Name != "Integration Test" {
		t.Errorf("GetByID().Name = %q, want %q", got.Name, "Integration Test")
	}
	if got.Email != "integration@test.com" {
		t.Errorf("GetByID().Email = %q, want %q", got.Email, "integration@test.com")
	}
}

func TestUserStore_DuplicateEmail(t *testing.T) {
	ctx := context.Background()
	s := store.NewPostgresUserStore(testDB)

	t.Cleanup(func() {
		_, _ = testDB.ExecContext(ctx, "DELETE FROM users WHERE email = $1", "duplicate@test.com")
	})

	_, err := s.Create(ctx, store.CreateUserInput{
		Name:  "First",
		Email: "duplicate@test.com",
	})
	if err != nil {
		t.Fatalf("first Create() error: %v", err)
	}

	_, err = s.Create(ctx, store.CreateUserInput{
		Name:  "Second",
		Email: "duplicate@test.com",
	})
	if err == nil {
		t.Fatal("second Create() expected error for duplicate email, got nil")
	}
}
```

### Testing with Redis via testcontainers

```go
//go:build integration

package cache_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var redisClient *redis.Client

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:7-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(15 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		log.Fatalf("failed to start redis: %v", err)
	}

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "6379")

	redisClient = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", host, port.Port()),
	})

	code := m.Run()

	redisClient.Close()
	_ = container.Terminate(ctx)
	os.Exit(code)
}

func TestCache_SetAndGet(t *testing.T) {
	ctx := context.Background()

	t.Cleanup(func() {
		redisClient.FlushDB(ctx)
	})

	err := redisClient.Set(ctx, "key1", "value1", 10*time.Second).Err()
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	val, err := redisClient.Get(ctx, "key1").Result()
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != "value1" {
		t.Errorf("Get() = %q, want %q", val, "value1")
	}
}
```

## Benefits

1. **Real dependency validation** - Catches bugs that mocks hide: SQL errors, constraint violations, serialization issues.
2. **Build tag isolation** - Unit tests stay fast; integration tests run only when explicitly requested.
3. **Reproducible environments** - testcontainers-go spins up identical containers every time, no manual setup.
4. **Shared setup via TestMain** - Expensive resources (containers, connections) are created once per package.
5. **CI-friendly** - Tests are self-contained; CI only needs Docker installed.

## Best Practices

- Always use build tags (`//go:build integration`) rather than relying solely on `testing.Short()`.
- Use `t.Cleanup()` to delete test data after each test rather than relying on a clean database.
- Keep integration tests in the same package as the code they test, with `_integration_test.go` suffix or build tag.
- Run integration tests with a timeout: `go test -tags=integration -timeout 5m ./...`.
- Use `t.Parallel()` cautiously in integration tests; ensure tests do not share mutable state.
- Name containers deterministically or use `testcontainers.GenericContainer` with `Reuse: true` for faster local iteration.
- In CI, run unit tests and integration tests as separate pipeline stages.

## Anti-Patterns

### Do not mix unit and integration tests without separation

Running integration tests on every `go test ./...` slows down feedback loops and causes failures when Docker is unavailable.

### Do not share mutable database state between tests

Each test should set up and clean up its own data. Relying on insertion order or test execution order creates flaky tests.

### Do not hardcode connection strings

Always derive connection details from the container at runtime. Hardcoded ports will conflict in parallel CI runs.

### Do not skip cleanup

Leaking containers or database rows causes test pollution. Always use `t.Cleanup()` or `defer` for resource cleanup, and terminate containers in `TestMain`.

## Related Patterns

- [core-sdk-go.testing-unit](./core-sdk-go.testing-unit.md) - Unit tests that run without external dependencies
- [core-sdk-go.testing-mocks](./core-sdk-go.testing-mocks.md) - Mock-based isolation when integration tests are too slow
- [core-sdk-go.testing-fixtures](./core-sdk-go.testing-fixtures.md) - Test data setup and factory patterns

## Testing (meta: how to test this pattern)

Run integration tests only:
```bash
go test -tags=integration -v ./...
```

Run integration tests with timeout and race detector:
```bash
go test -tags=integration -race -timeout 5m ./...
```

Skip integration tests (default behavior):
```bash
go test ./...
```

Skip long tests explicitly:
```bash
go test -short ./...
```

---

**Status**: Active
**Compatibility**: Go 1.21+
