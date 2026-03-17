# Pattern: Test Fixtures

**Namespace**: core-sdk-go
**Category**: Testing
**Created**: 2026-03-17
**Status**: Active

---

## Overview

Test fixtures in Go encompass factory functions for creating test data, the `testdata/` directory convention for static test files, and the golden file pattern for snapshot testing. These techniques reduce duplication, make tests self-documenting, and provide reliable mechanisms for validating complex output.

## Problem

Tests that inline large struct literals or hard-code expected output become noisy and hard to maintain. When the same entity appears in dozens of tests, a field change requires updating every test. Complex outputs (JSON, HTML, protocol buffers) are impractical to assert field-by-field.

## Solution

Use factory functions to construct test entities with sensible defaults and per-test overrides. Store static test input in the `testdata/` directory (which Go tooling ignores during builds). Use golden files to snapshot expected output and compare against it, with an update flag for intentional changes.

## Implementation

### The testdata/ Directory

Go tooling ignores directories named `testdata/`. Place test input files, fixtures, and golden files here:

```
mypackage/
    parser.go
    parser_test.go
    testdata/
        valid_input.json
        invalid_input.json
        golden/
            expected_output.json
```

Loading test data:

```go
package parser_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/example/project/parser"
)

func TestParse_ValidInput(t *testing.T) {
	input, err := os.ReadFile(filepath.Join("testdata", "valid_input.json"))
	if err != nil {
		t.Fatalf("failed to read test data: %v", err)
	}

	result, err := parser.Parse(input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if result.Name != "expected" {
		t.Errorf("result.Name = %q, want %q", result.Name, "expected")
	}
}
```

### Factory Functions

Create test entities with sensible defaults. Override only what matters for each test:

```go
package testutil

import (
	"time"

	"github.com/example/project/model"
)

// UserOption configures a test user via functional options.
type UserOption func(*model.User)

func WithName(name string) UserOption {
	return func(u *model.User) { u.Name = name }
}

func WithEmail(email string) UserOption {
	return func(u *model.User) { u.Email = email }
}

func WithActive(active bool) UserOption {
	return func(u *model.User) { u.Active = active }
}

func WithRole(role string) UserOption {
	return func(u *model.User) { u.Role = role }
}

// NewTestUser creates a user with sensible defaults. Pass options to override.
func NewTestUser(opts ...UserOption) *model.User {
	u := &model.User{
		ID:        "test-user-1",
		Name:      "Test User",
		Email:     "test@example.com",
		Active:    true,
		Role:      "member",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	for _, opt := range opts {
		opt(u)
	}
	return u
}
```

Using the factory in tests:

```go
package order_test

import (
	"context"
	"testing"

	"github.com/example/project/order"
	"github.com/example/project/testutil"
)

func TestPlaceOrder_InactiveUser(t *testing.T) {
	user := testutil.NewTestUser(
		testutil.WithActive(false),
	)

	svc := order.NewService(&mockUserFetcher{user: user})
	_, err := svc.PlaceOrder(context.Background(), user.ID, nil)
	if err == nil {
		t.Fatal("expected error for inactive user")
	}
}

func TestPlaceOrder_AdminUser(t *testing.T) {
	user := testutil.NewTestUser(
		testutil.WithRole("admin"),
	)

	svc := order.NewService(&mockUserFetcher{user: user})
	o, err := svc.PlaceOrder(context.Background(), user.ID, []order.Item{{SKU: "ABC"}})
	if err != nil {
		t.Fatalf("PlaceOrder() error: %v", err)
	}
	if !o.PriorityProcessing {
		t.Error("admin order should have priority processing")
	}
}
```

### Simple Factory Without Options

For smaller structs, a direct function with parameters is simpler:

```go
package testutil

import "github.com/example/project/model"

func NewTestOrder(userID string, itemCount int) *model.Order {
	items := make([]model.Item, itemCount)
	for i := range items {
		items[i] = model.Item{
			SKU:      fmt.Sprintf("ITEM-%03d", i+1),
			Quantity: 1,
			Price:    999, // cents
		}
	}
	return &model.Order{
		ID:     "test-order-1",
		UserID: userID,
		Items:  items,
		Status: model.OrderStatusPending,
	}
}
```

### Test Helpers with t.Helper()

```go
package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// LoadJSON reads a JSON file from testdata/ and unmarshals it into dest.
func LoadJSON(t *testing.T, filename string, dest interface{}) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", filename))
	if err != nil {
		t.Fatalf("LoadJSON(%q): %v", filename, err)
	}
	if err := json.Unmarshal(data, dest); err != nil {
		t.Fatalf("LoadJSON(%q) unmarshal: %v", filename, err)
	}
}

// LoadBytes reads a file from testdata/ and returns its contents.
func LoadBytes(t *testing.T, filename string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", filename))
	if err != nil {
		t.Fatalf("LoadBytes(%q): %v", filename, err)
	}
	return data
}
```

### Golden File Pattern

Golden files store expected output as files. Tests compare actual output against the golden file. Pass `-update` to regenerate golden files when output intentionally changes.

```go
package renderer_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/example/project/renderer"
)

var update = flag.Bool("update", false, "update golden files")

func TestRender_GoldenFile(t *testing.T) {
	tests := []struct {
		name  string
		input renderer.Input
	}{
		{
			name:  "simple_page",
			input: renderer.Input{Title: "Hello", Body: "World"},
		},
		{
			name:  "page_with_metadata",
			input: renderer.Input{Title: "Hello", Body: "World", Tags: []string{"go", "test"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderer.Render(tt.input)
			goldenPath := filepath.Join("testdata", "golden", tt.name+".html")

			if *update {
				// Write the actual output as the new golden file
				err := os.MkdirAll(filepath.Dir(goldenPath), 0o755)
				if err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				err = os.WriteFile(goldenPath, []byte(got), 0o644)
				if err != nil {
					t.Fatalf("write golden file: %v", err)
				}
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden file (run with -update to create): %v", err)
			}
			if got != string(want) {
				t.Errorf("output mismatch (run with -update to regenerate)\ngot:\n%s\nwant:\n%s", got, string(want))
			}
		})
	}
}
```

### Golden File for JSON Output

```go
package api_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/example/project/api"
)

var update = flag.Bool("update", false, "update golden files")

func assertGoldenJSON(t *testing.T, name string, got interface{}) {
	t.Helper()

	actual, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal actual: %v", err)
	}
	// Append trailing newline for clean diffs
	actual = append(actual, '\n')

	goldenPath := filepath.Join("testdata", "golden", name+".json")

	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(goldenPath, actual, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if string(actual) != string(want) {
		t.Errorf("golden mismatch for %s\ngot:\n%s\nwant:\n%s", name, actual, want)
	}
}

func TestListUsers_Response(t *testing.T) {
	resp := api.ListUsers()
	assertGoldenJSON(t, "list_users_response", resp)
}
```

## Benefits

1. **Reduced duplication** - Factory functions centralize test entity construction; a field change updates one place.
2. **Self-documenting tests** - `WithActive(false)` reads better than setting 15 struct fields where only one matters.
3. **Stable test data** - `testdata/` files are versioned and reviewable in pull requests.
4. **Snapshot confidence** - Golden files catch unintended output changes automatically.
5. **Build-tool friendly** - Go ignores `testdata/` during compilation and installation.

## Best Practices

- Place all static test files in `testdata/` -- Go build, vet, and module tools ignore this directory.
- Use functional options for factories when structs have many fields.
- Use simple parameters for factories when structs are small or only 1-2 fields vary.
- Commit golden files to version control. Review golden file diffs in PRs to catch unintended changes.
- Use the `-update` flag pattern so golden files are easy to regenerate intentionally.
- Name golden files after the test case for easy traceability.
- Use `t.Helper()` in all fixture and assertion helper functions.
- Consider a shared `testutil` or `internal/testutil` package for factories used across multiple packages.

## Anti-Patterns

### Do not inline large struct literals repeatedly

```go
// Bad: duplicated across 20 tests
user := &model.User{
	ID: "1", Name: "Alice", Email: "alice@test.com",
	Active: true, Role: "member", CreatedAt: time.Now(),
}

// Good: factory with defaults
user := testutil.NewTestUser()
```

### Do not use golden files for volatile output

If output contains timestamps, random IDs, or non-deterministic ordering, normalize or strip those fields before comparison. Otherwise golden tests will be flaky.

### Do not put test code in testdata/

The `testdata/` directory is for data files (JSON, SQL, HTML, binary fixtures). Test code belongs in `*_test.go` files.

### Do not forget to commit golden file updates

If you run `-update` locally but forget to commit the changed golden files, CI will fail. Always review and commit golden file changes together with the code that changed them.

## Related Patterns

- [core-sdk-go.testing-unit](./core-sdk-go.testing-unit.md) - Table-driven tests that use these fixtures
- [core-sdk-go.testing-mocks](./core-sdk-go.testing-mocks.md) - Mocks that return factory-created test data
- [core-sdk-go.testing-integration](./core-sdk-go.testing-integration.md) - Integration tests that load testdata files

## Testing (meta: how to test this pattern)

Run tests normally:
```bash
go test -v ./...
```

Update golden files after intentional output changes:
```bash
go test -v -update ./...
```

Verify golden files are up to date in CI (no `-update` flag):
```bash
go test ./...
```

---

**Status**: Active
**Compatibility**: Go 1.21+
