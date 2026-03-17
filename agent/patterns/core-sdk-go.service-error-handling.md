# Pattern: Service Error Handling

**Namespace**: core-sdk-go
**Category**: Error Handling
**Created**: 2026-03-17
**Status**: Active

---

## Overview

Go errors are values, not exceptions. This pattern establishes conventions for sentinel errors, custom error types, error wrapping, and error inspection using the standard library's `errors.Is()` and `errors.As()`. It replaces exception hierarchies with composable error types that carry severity, context, and domain information.

---

## Problem

Go's `(T, error)` return pattern is simple but requires discipline. Without conventions, codebases accumulate inconsistent error handling: string comparison instead of sentinel checks, lost context from bare `return err`, and no structured way to convey error severity or domain. Services need a consistent approach to creating, wrapping, inspecting, and responding to errors.

---

## Solution

Use three complementary techniques: sentinel errors for well-known conditions (`ErrNotFound`, `ErrUnauthorized`), custom error types for domain-specific context (`ValidationError`, `ServiceError`), and `fmt.Errorf` with `%w` for wrapping errors with call-site context. Consumers use `errors.Is()` for sentinel checks and `errors.As()` for type extraction.

---

## Implementation

### Go Implementation

#### Sentinel Errors

```go
package apperr

import "errors"

// Sentinel errors for well-known conditions.
// These are package-level variables, compared with errors.Is().
var (
	ErrNotFound      = errors.New("not found")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrForbidden     = errors.New("forbidden")
	ErrConflict      = errors.New("conflict")
	ErrInvalidInput  = errors.New("invalid input")
	ErrInternal      = errors.New("internal error")
	ErrUnavailable   = errors.New("service unavailable")
)
```

#### Custom Error Types

```go
package apperr

import "fmt"

// Severity indicates how critical an error is.
type Severity int

const (
	SeverityLow Severity = iota
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

func (s Severity) String() string {
	switch s {
	case SeverityLow:
		return "low"
	case SeverityMedium:
		return "medium"
	case SeverityHigh:
		return "high"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// ValidationError represents input validation failures.
type ValidationError struct {
	Field   string
	Message string
	Value   any
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed on field %q: %s", e.Field, e.Message)
}

// NewValidationError creates a ValidationError.
func NewValidationError(field, message string, value any) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	}
}

// NotFoundError indicates a specific resource was not found.
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s with id %q not found", e.Resource, e.ID)
}

// Unwrap allows errors.Is(err, ErrNotFound) to work.
func (e *NotFoundError) Unwrap() error {
	return ErrNotFound
}

// NewNotFoundError creates a NotFoundError.
func NewNotFoundError(resource, id string) *NotFoundError {
	return &NotFoundError{
		Resource: resource,
		ID:       id,
	}
}

// ServiceError represents an error from a specific service with severity.
type ServiceError struct {
	Service  string
	Op       string
	Severity Severity
	Err      error
}

func (e *ServiceError) Error() string {
	return fmt.Sprintf("[%s] %s.%s: %v", e.Severity, e.Service, e.Op, e.Err)
}

// Unwrap supports errors.Is() and errors.As() through the chain.
func (e *ServiceError) Unwrap() error {
	return e.Err
}

// NewServiceError creates a ServiceError wrapping an underlying error.
func NewServiceError(service, op string, severity Severity, err error) *ServiceError {
	return &ServiceError{
		Service:  service,
		Op:       op,
		Severity: severity,
		Err:      err,
	}
}
```

#### Error Wrapping at Call Sites

```go
package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"myapp/apperr"
)

func (s *UserService) GetByID(ctx context.Context, id string) (*User, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		// Wrap with context about what operation failed
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NewNotFoundError("user", id)
		}
		return nil, apperr.NewServiceError(
			"UserService", "GetByID", apperr.SeverityHigh, err,
		)
	}
	return u, nil
}

func (s *UserService) Create(ctx context.Context, name, email string) (*User, error) {
	// Validate input
	if name == "" {
		return nil, apperr.NewValidationError("name", "must not be empty", name)
	}
	if email == "" {
		return nil, apperr.NewValidationError("email", "must not be empty", email)
	}

	u := &User{Name: name, Email: email}
	if err := s.repo.Save(ctx, u); err != nil {
		// Wrap with fmt.Errorf %w for simpler cases
		return nil, fmt.Errorf("saving user: %w", err)
	}
	return u, nil
}
```

#### Error Inspection

```go
package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"myapp/apperr"
)

// ErrorToHTTPStatus maps application errors to HTTP status codes.
func ErrorToHTTPStatus(err error) int {
	// Check sentinel errors with errors.Is
	switch {
	case errors.Is(err, apperr.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, apperr.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, apperr.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, apperr.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, apperr.ErrInvalidInput):
		return http.StatusBadRequest
	}

	// Check custom error types with errors.As
	var validationErr *apperr.ValidationError
	if errors.As(err, &validationErr) {
		return http.StatusBadRequest
	}

	var serviceErr *apperr.ServiceError
	if errors.As(err, &serviceErr) {
		if serviceErr.Severity >= apperr.SeverityCritical {
			return http.StatusServiceUnavailable
		}
		return http.StatusInternalServerError
	}

	return http.StatusInternalServerError
}

// HandleError logs and responds with an appropriate HTTP error.
func HandleError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	status := ErrorToHTTPStatus(err)

	// Log with severity context if available
	var serviceErr *apperr.ServiceError
	if errors.As(err, &serviceErr) {
		logger.ErrorContext(r.Context(), "service error",
			"service", serviceErr.Service,
			"op", serviceErr.Op,
			"severity", serviceErr.Severity.String(),
			"error", err,
		)
	} else {
		logger.ErrorContext(r.Context(), "request error",
			"status", status,
			"error", err,
		)
	}

	http.Error(w, http.StatusText(status), status)
}
```

### Example Usage

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"myapp/apperr"
	"myapp/user"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	svc := user.NewUserService(/* ... */)

	ctx := context.Background()
	u, err := svc.GetByID(ctx, "nonexistent-id")
	if err != nil {
		// Check for specific error type
		var notFound *apperr.NotFoundError
		if errors.As(err, &notFound) {
			logger.Info("user not found",
				"resource", notFound.Resource,
				"id", notFound.ID,
			)
			return
		}

		// Check for sentinel
		if errors.Is(err, apperr.ErrUnauthorized) {
			logger.Warn("unauthorized access attempt")
			return
		}

		// Unknown error
		logger.Error("unexpected error", "error", err)
		return
	}

	fmt.Printf("Found user: %s\n", u.Name)
}
```

---

## Benefits

1. Errors are values -- they compose, wrap, and inspect without try/catch overhead
2. Sentinel errors provide stable, comparable conditions across packages
3. Custom error types carry domain context (resource, field, severity)
4. `Unwrap()` enables `errors.Is()` to traverse the full error chain
5. `errors.As()` extracts typed information without type assertions on wrapped errors
6. No exception hierarchy to maintain -- error types are independent structs
7. Compile-time safety -- the `Error()` method is checked at compile time

---

## Best Practices

**Do wrap errors with context at each call site:**
```go
user, err := s.repo.FindByID(ctx, id)
if err != nil {
    return nil, fmt.Errorf("UserService.GetByID(%s): %w", id, err)
}
```

**Do implement Unwrap() to enable error chain traversal:**
```go
func (e *NotFoundError) Unwrap() error {
    return ErrNotFound // enables errors.Is(err, ErrNotFound)
}
```

**Do use errors.Is() for sentinel checks, errors.As() for type checks:**
```go
if errors.Is(err, apperr.ErrNotFound) { /* handle not found */ }

var ve *apperr.ValidationError
if errors.As(err, &ve) {
    log.Printf("invalid field: %s", ve.Field)
}
```

**Do define sentinel errors as package-level vars:**
```go
var ErrNotFound = errors.New("not found")
```

---

## Anti-Patterns

**Don't compare error strings:**
```go
// BAD: fragile, breaks if message changes
if err.Error() == "not found" {
    // ...
}

// GOOD: use sentinel or type check
if errors.Is(err, apperr.ErrNotFound) {
    // ...
}
```

**Don't discard errors silently:**
```go
// BAD: swallows the error -- bugs become invisible
result, _ := svc.GetByID(ctx, id)

// GOOD: always handle the error
result, err := svc.GetByID(ctx, id)
if err != nil {
    return fmt.Errorf("getting user: %w", err)
}
```

**Don't use panic for expected error conditions:**
```go
// BAD: panics crash the program for recoverable situations
func MustGetUser(ctx context.Context, id string) *User {
    u, err := repo.FindByID(ctx, id)
    if err != nil {
        panic(err) // Don't do this for normal errors
    }
    return u
}

// GOOD: return the error and let the caller decide
func GetUser(ctx context.Context, id string) (*User, error) {
    return repo.FindByID(ctx, id)
}
```

**Don't wrap without adding context:**
```go
// BAD: wrapping adds nothing -- just noise
if err != nil {
    return fmt.Errorf("%w", err)
}

// GOOD: add what operation failed
if err != nil {
    return fmt.Errorf("querying user %s: %w", id, err)
}
```

**Don't create error types for every function:**
```go
// BAD: over-engineering -- one error type per function
type GetUserByIDError struct { ... }
type ListUsersError struct { ... }
type CreateUserError struct { ... }

// GOOD: shared types for categories of errors
type NotFoundError struct { ... }
type ValidationError struct { ... }
```

---

## Related Patterns

- [Service Base](core-sdk-go.service-base.md) -- services use these error types in lifecycle methods
- [Service Interface](core-sdk-go.service-interface.md) -- error interfaces for cross-package contracts
- [Service Container](core-sdk-go.service-container.md) -- propagating init/close errors
- [Service Logging](core-sdk-go.service-logging.md) -- structured logging of error context

---

## Testing

### Unit Test Example

```go
package apperr_test

import (
	"errors"
	"fmt"
	"testing"

	"myapp/apperr"
)

func TestNotFoundError_ErrorsIs(t *testing.T) {
	err := apperr.NewNotFoundError("user", "abc-123")

	// errors.Is works through Unwrap
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Error("expected errors.Is to match ErrNotFound")
	}

	// Wrapping preserves the chain
	wrapped := fmt.Errorf("handler: %w", err)
	if !errors.Is(wrapped, apperr.ErrNotFound) {
		t.Error("expected wrapped error to still match ErrNotFound")
	}
}

func TestNotFoundError_ErrorsAs(t *testing.T) {
	original := apperr.NewNotFoundError("order", "ord-456")
	wrapped := fmt.Errorf("processing: %w", original)

	var notFound *apperr.NotFoundError
	if !errors.As(wrapped, &notFound) {
		t.Fatal("expected errors.As to extract NotFoundError")
	}
	if notFound.Resource != "order" {
		t.Errorf("expected resource 'order', got %q", notFound.Resource)
	}
	if notFound.ID != "ord-456" {
		t.Errorf("expected id 'ord-456', got %q", notFound.ID)
	}
}

func TestValidationError_Message(t *testing.T) {
	err := apperr.NewValidationError("email", "must contain @", "bad-email")

	expected := `validation failed on field "email": must contain @`
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestServiceError_Severity(t *testing.T) {
	inner := errors.New("connection refused")
	err := apperr.NewServiceError("UserService", "Init", apperr.SeverityCritical, inner)

	if err.Severity != apperr.SeverityCritical {
		t.Errorf("expected critical severity, got %s", err.Severity)
	}

	// Unwrap reaches the inner error
	if !errors.Is(err, inner) {
		t.Error("expected errors.Is to find inner error")
	}
}

func TestServiceError_WrappedNotFound(t *testing.T) {
	notFound := apperr.NewNotFoundError("product", "p-789")
	svcErr := apperr.NewServiceError(
		"ProductService", "GetByID", apperr.SeverityMedium, notFound,
	)

	// errors.Is traverses: ServiceError -> NotFoundError -> ErrNotFound
	if !errors.Is(svcErr, apperr.ErrNotFound) {
		t.Error("expected errors.Is to traverse chain to ErrNotFound")
	}

	// errors.As can still extract the NotFoundError
	var nf *apperr.NotFoundError
	if !errors.As(svcErr, &nf) {
		t.Error("expected errors.As to extract NotFoundError from chain")
	}
}

func TestErrorToHTTPStatus(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{"not found sentinel", apperr.ErrNotFound, 404},
		{"not found typed", apperr.NewNotFoundError("x", "1"), 404},
		{"validation error", apperr.NewValidationError("f", "bad", nil), 400},
		{"unauthorized", apperr.ErrUnauthorized, 401},
		{"unknown error", errors.New("something"), 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.ErrorToHTTPStatus(tt.err)
			if got != tt.status {
				t.Errorf("expected %d, got %d", tt.status, got)
			}
		})
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
