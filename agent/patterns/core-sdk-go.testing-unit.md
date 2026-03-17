# Pattern: Unit Testing

**Namespace**: core-sdk-go
**Category**: Testing
**Created**: 2026-03-17
**Status**: Active

---

## Overview

Go unit testing with the standard `testing` package using table-driven tests, subtests, and test helpers. This pattern covers idiomatic Go test organization, naming conventions, and the Arrange-Act-Assert structure that keeps tests readable and maintainable.

## Problem

Without a consistent unit testing approach, Go projects accumulate tests that are hard to read, duplicate setup logic, and fail to cover edge cases systematically. Teams coming from other languages may reach for heavy assertion frameworks or test runners when the standard library already provides everything needed.

## Solution

Use Go's built-in `testing` package with table-driven tests as the primary unit testing pattern. Organize tests next to production code, use subtests for granularity, and leverage `t.Helper()` for clean failure output.

## Implementation

### File Organization

```
mypackage/
    user.go
    user_test.go          # Black-box tests (package mypackage_test)
    user_internal_test.go  # White-box tests (package mypackage)
```

### Package Naming

Use `package foo_test` for black-box testing (tests the public API only). Use `package foo` for white-box testing when you need access to unexported internals.

```go
// user_test.go — black-box: tests only exported API
package mypackage_test

import (
	"testing"

	"github.com/example/project/mypackage"
)

func TestNewUser(t *testing.T) {
	u := mypackage.NewUser("alice", "alice@example.com")
	if u.Name() != "alice" {
		t.Errorf("expected name %q, got %q", "alice", u.Name())
	}
}
```

```go
// user_internal_test.go — white-box: can access unexported fields
package mypackage

import "testing"

func TestNormalizeEmail(t *testing.T) {
	got := normalizeEmail("  Alice@Example.COM  ")
	want := "alice@example.com"
	if got != want {
		t.Errorf("normalizeEmail() = %q, want %q", got, want)
	}
}
```

### Table-Driven Tests

The idiomatic Go pattern for covering multiple cases concisely:

```go
package calculator_test

import (
	"testing"

	"github.com/example/project/calculator"
)

func TestAdd(t *testing.T) {
	tests := []struct {
		name string
		a, b int
		want int
	}{
		{name: "positive numbers", a: 2, b: 3, want: 5},
		{name: "negative numbers", a: -1, b: -2, want: -3},
		{name: "mixed sign", a: -1, b: 5, want: 4},
		{name: "zeros", a: 0, b: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculator.Add(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("Add(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
```

### Testing Error Returns

```go
package store_test

import (
	"errors"
	"testing"

	"github.com/example/project/store"
)

func TestGetUser(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		want    *store.User
		wantErr error
	}{
		{
			name: "existing user",
			id:   "user-1",
			want: &store.User{ID: "user-1", Name: "Alice"},
		},
		{
			name:    "missing user",
			id:      "user-999",
			wantErr: store.ErrNotFound,
		},
		{
			name:    "empty id",
			id:      "",
			wantErr: store.ErrInvalidID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := store.NewMemoryStore()
			s.Seed(store.User{ID: "user-1", Name: "Alice"})

			got, err := s.GetUser(tt.id)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("GetUser(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetUser(%q) unexpected error: %v", tt.id, err)
			}
			if got.ID != tt.want.ID || got.Name != tt.want.Name {
				t.Errorf("GetUser(%q) = %+v, want %+v", tt.id, got, tt.want)
			}
		})
	}
}
```

### Test Helpers with t.Helper()

```go
package testutil

import (
	"testing"

	"github.com/example/project/store"
)

// MustCreateUser creates a user or fails the test. The t.Helper() call
// ensures the failure message points to the caller, not this function.
func MustCreateUser(t *testing.T, s *store.MemoryStore, name, email string) *store.User {
	t.Helper()
	u, err := s.CreateUser(name, email)
	if err != nil {
		t.Fatalf("MustCreateUser(%q, %q): %v", name, email, err)
	}
	return u
}
```

### Arrange-Act-Assert (AAA) Pattern

```go
func TestTransferFunds(t *testing.T) {
	// Arrange
	bank := NewBank()
	from := bank.CreateAccount("Alice", 1000)
	to := bank.CreateAccount("Bob", 500)

	// Act
	err := bank.Transfer(from.ID, to.ID, 250)

	// Assert
	if err != nil {
		t.Fatalf("Transfer() unexpected error: %v", err)
	}
	if from.Balance() != 750 {
		t.Errorf("sender balance = %d, want 750", from.Balance())
	}
	if to.Balance() != 750 {
		t.Errorf("receiver balance = %d, want 750", to.Balance())
	}
}
```

### Using testify (Optional)

While Go's standard library is sufficient, `testify` is widely accepted:

```go
package calculator_test

import (
	"testing"

	"github.com/example/project/calculator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDivide(t *testing.T) {
	result, err := calculator.Divide(10, 2)
	require.NoError(t, err)
	assert.Equal(t, 5.0, result)

	_, err = calculator.Divide(10, 0)
	assert.ErrorIs(t, err, calculator.ErrDivideByZero)
}
```

## Benefits

1. **No external dependencies** - The `testing` package ships with Go; no third-party test runner needed.
2. **Table-driven tests scale** - Adding a new case is a single struct literal; no boilerplate.
3. **Subtests enable selective runs** - `go test -run TestAdd/zeros` runs one case.
4. **t.Helper() keeps output clean** - Failures point to the caller, not the helper.
5. **Parallel-friendly** - Add `t.Parallel()` to subtests for concurrent execution.

## Best Practices

- Name test files `foo_test.go` next to `foo.go`.
- Prefer `package foo_test` for testing the public API; use `package foo` only when testing unexported logic.
- Use `t.Run()` for subtests so each case gets its own name in output.
- Call `t.Helper()` in every test helper function.
- Use `t.Fatalf()` for setup failures that make the rest of the test meaningless; use `t.Errorf()` for assertion failures where you want the test to continue.
- Keep the `tt` variable name convention for table-driven test loop variables.
- Use `t.Parallel()` in subtests when tests are independent and you want faster execution.
- Use `t.Cleanup()` for teardown instead of `defer` when the cleanup should happen after the test and all its subtests complete.

## Anti-Patterns

### Do not use if/else chains instead of table-driven tests

```go
// Bad: duplicated test logic
func TestAdd(t *testing.T) {
	if calculator.Add(1, 2) != 3 {
		t.Error("1+2 failed")
	}
	if calculator.Add(0, 0) != 0 {
		t.Error("0+0 failed")
	}
	// ... more copy-paste
}
```

### Do not skip t.Helper() in helpers

Without `t.Helper()`, failure messages point to the helper function rather than the test that called it, making debugging harder.

### Do not use fmt.Println for debugging

Use `t.Logf()` instead. Output from `t.Log` only appears when the test fails or when running with `-v`.

### Do not test multiple behaviors in one test function

Split distinct behaviors into separate `t.Run` subtests or separate test functions.

## Related Patterns

- [core-sdk-go.testing-mocks](./core-sdk-go.testing-mocks.md) - Mocking dependencies for isolated unit tests
- [core-sdk-go.testing-fixtures](./core-sdk-go.testing-fixtures.md) - Test data factories and golden files
- [core-sdk-go.testing-integration](./core-sdk-go.testing-integration.md) - When unit tests are not enough

## Testing (meta: how to test this pattern)

Run all unit tests in a package:
```bash
go test ./mypackage/...
```

Run a specific test:
```bash
go test ./mypackage/ -run TestAdd
```

Run a specific subtest:
```bash
go test ./mypackage/ -run TestAdd/zeros
```

Run with verbose output:
```bash
go test -v ./mypackage/...
```

Run with race detector:
```bash
go test -race ./mypackage/...
```

---

**Status**: Active
**Compatibility**: Go 1.21+
