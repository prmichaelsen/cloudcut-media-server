# Pattern: Test Coverage

**Namespace**: core-sdk-go
**Category**: Testing
**Created**: 2026-03-17
**Status**: Active

---

## Overview

Go has built-in test coverage tooling via `go test -cover`. This pattern covers generating coverage profiles, viewing HTML reports, setting CI thresholds, handling concurrent test coverage correctly, and strategies for per-package vs whole-project coverage measurement.

## Problem

Without coverage tracking, teams have no objective measure of which code paths are tested. Untested code accumulates silently, particularly error handling, edge cases, and rarely exercised branches. CI pipelines that do not enforce coverage thresholds allow regressions to slip through.

## Solution

Use `go test -coverprofile` to generate coverage data, `go tool cover -html` to visualize it, and CI scripts to enforce minimum thresholds. Use `-covermode=atomic` for projects with concurrent tests. Integrate coverage into the development workflow so gaps are caught early.

## Implementation

### Basic Coverage

```bash
# Run tests with coverage summary
go test -cover ./...

# Output example:
# ok  github.com/example/project/store   0.023s  coverage: 84.2% of statements
# ok  github.com/example/project/handler  0.031s  coverage: 72.1% of statements
```

### Coverage Profile and HTML Report

```bash
# Generate a coverage profile
go test -coverprofile=coverage.out ./...

# View as HTML report in your browser
go tool cover -html=coverage.out -o coverage.html

# View function-level coverage in the terminal
go tool cover -func=coverage.out
```

Output from `-func`:

```
github.com/example/project/store/user.go:15:     NewUserStore     100.0%
github.com/example/project/store/user.go:23:     Create           87.5%
github.com/example/project/store/user.go:45:     GetByID          100.0%
github.com/example/project/store/user.go:62:     Delete           66.7%
total:                                            (statements)     85.3%
```

### Coverage Modes

Go supports three coverage modes:

```bash
# set: did this statement run? (default, fastest)
go test -covermode=set -coverprofile=coverage.out ./...

# count: how many times did this statement run?
go test -covermode=count -coverprofile=coverage.out ./...

# atomic: like count but safe for concurrent tests (use with -race)
go test -covermode=atomic -coverprofile=coverage.out ./...
```

Use `-covermode=atomic` whenever your tests use `t.Parallel()` or test concurrent code. It uses sync/atomic operations to avoid data races on the coverage counters.

### Per-Package vs Whole-Project Coverage

By default, `go test -cover` measures coverage per package -- each package's tests only count coverage of that same package.

To measure cross-package coverage (e.g., integration tests in package A that exercise code in package B):

```bash
# Cover all packages, even those exercised by tests in other packages
go test -coverprofile=coverage.out -coverpkg=./... ./...
```

### Complete CI Pipeline

```bash
#!/bin/bash
set -euo pipefail

THRESHOLD=80

# Run tests with atomic coverage across all packages
go test \
  -race \
  -covermode=atomic \
  -coverprofile=coverage.out \
  -coverpkg=./... \
  ./...

# Display function-level coverage
go tool cover -func=coverage.out

# Extract total coverage percentage
COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')

echo "Total coverage: ${COVERAGE}%"

# Enforce threshold
if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
  echo "FAIL: Coverage ${COVERAGE}% is below threshold ${THRESHOLD}%"
  exit 1
fi

echo "PASS: Coverage ${COVERAGE}% meets threshold ${THRESHOLD}%"

# Generate HTML report for artifacts
go tool cover -html=coverage.out -o coverage.html
```

### GitHub Actions Workflow

```yaml
name: Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.21'

      - name: Run tests with coverage
        run: |
          go test \
            -race \
            -covermode=atomic \
            -coverprofile=coverage.out \
            -coverpkg=./... \
            ./...

      - name: Check coverage threshold
        run: |
          COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          echo "Total coverage: ${COVERAGE}%"
          if (( $(echo "$COVERAGE < 80" | bc -l) )); then
            echo "::error::Coverage ${COVERAGE}% is below 80% threshold"
            exit 1
          fi

      - name: Generate HTML report
        if: always()
        run: go tool cover -html=coverage.out -o coverage.html

      - name: Upload coverage report
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: coverage-report
          path: coverage.html
```

### Excluding Files from Coverage

Go does not have a built-in exclude mechanism, but there are several approaches:

#### 1. Exclude with grep on the profile

```bash
# Generate coverage
go test -coverprofile=coverage.out ./...

# Remove generated code and test helpers from the profile
grep -v -E '(\.pb\.go|_generated\.go|/mocks/|/testutil/)' coverage.out > coverage.filtered.out

# Report on filtered coverage
go tool cover -func=coverage.filtered.out
```

#### 2. Exclude specific packages from the test run

```bash
# List all packages, filter out ones you want to exclude, then test
go test -coverprofile=coverage.out $(go list ./... | grep -v -E '(generated|mocks|cmd)')
```

#### 3. Use build tags to separate code

```go
//go:build !coverage_exclude

package main

// This file is excluded when running: go test -tags=coverage_exclude ./...
```

### Makefile Integration

```makefile
.PHONY: test test-cover test-cover-html

COVERAGE_THRESHOLD := 80

test:
	go test -race ./...

test-cover:
	go test -race -covermode=atomic -coverprofile=coverage.out -coverpkg=./... ./...
	@COVERAGE=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "Coverage: $${COVERAGE}%"; \
	if [ $$(echo "$${COVERAGE} < $(COVERAGE_THRESHOLD)" | bc -l) -eq 1 ]; then \
		echo "FAIL: Coverage below $(COVERAGE_THRESHOLD)%"; \
		exit 1; \
	fi

test-cover-html: test-cover
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"
```

### Viewing Coverage for a Specific Package

```bash
# Coverage for a single package
go test -coverprofile=coverage.out ./store/...
go tool cover -func=coverage.out

# Coverage for a single file's functions
go tool cover -func=coverage.out | grep user.go
```

## Benefits

1. **Built-in tooling** - No external tools required; `go test -cover` and `go tool cover` ship with Go.
2. **Visual HTML reports** - Quickly identify untested code paths with color-coded source views.
3. **CI enforcement** - Automated threshold checks prevent coverage regressions.
4. **Granular reporting** - Function-level and statement-level coverage data for targeted improvement.
5. **Race-safe measurement** - `-covermode=atomic` works correctly with concurrent tests and `-race`.

## Best Practices

- Use `-covermode=atomic` in CI to avoid false results from concurrent tests.
- Use `-coverpkg=./...` to capture cross-package coverage from integration tests.
- Set a realistic threshold (70-80%) and increase it gradually rather than starting at 100%.
- Add `coverage.out` and `coverage.html` to `.gitignore`.
- Review coverage HTML reports during code review to verify new code is tested.
- Focus on covering critical paths (error handling, security checks) rather than chasing a number.
- Run coverage as part of the PR check, not just on main.

## Anti-Patterns

### Do not chase 100% coverage

100% line coverage does not mean correct code. Focus on meaningful tests that verify behavior. Getters, setters, and simple delegating functions often do not need dedicated tests.

### Do not use coverage as the only quality metric

High coverage with weak assertions (tests that run code but do not check results) provides false confidence. Combine coverage with mutation testing or thorough assertion practices.

### Do not ignore coverage drops

If a PR drops coverage by 5%, investigate. Either the new code is untested, or existing tests were removed. CI thresholds catch this automatically.

### Do not exclude too much from coverage

Excluding large portions of the codebase from coverage measurement defeats the purpose. Only exclude truly generated code (protobuf, mocks) and CLI entry points.

## Related Patterns

- [core-sdk-go.testing-unit](./core-sdk-go.testing-unit.md) - Unit tests that contribute to coverage
- [core-sdk-go.testing-integration](./core-sdk-go.testing-integration.md) - Integration tests that cover code paths unit tests miss
- [core-sdk-go.testing-mocks](./core-sdk-go.testing-mocks.md) - Mocks enable testing code paths that are hard to reach otherwise

## Testing (meta: how to test this pattern)

Quick coverage check:
```bash
go test -cover ./...
```

Full coverage with HTML report:
```bash
go test -race -covermode=atomic -coverprofile=coverage.out -coverpkg=./... ./...
go tool cover -html=coverage.out -o coverage.html
```

Check a specific threshold:
```bash
COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
echo "Coverage: ${COVERAGE}%"
```

---

**Status**: Active
**Compatibility**: Go 1.21+
