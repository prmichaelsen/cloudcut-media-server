# Pattern: Error Type System

**Namespace**: core-sdk-go
**Category**: Type System
**Created**: 2026-03-17
**Status**: Active

---

## Overview

Defines a structured error type hierarchy in Go using custom types that implement the `error` interface. This pattern provides error classification (kinds/codes), wrapping for context propagation, and adapter functions that map domain errors to transport-specific representations (HTTP status codes, CLI exit codes, gRPC codes). This is the **type-system view** of errors -- how error types are defined and composed. For the **usage view** (how services create, return, and handle errors), see the complementary service-error-handling pattern.

## Problem

Go's built-in `error` interface is intentionally minimal. Without a structured error type system:

- **No classification**: Callers cannot distinguish "not found" from "validation failed" from "internal error" without string parsing.
- **Lost context**: Using `fmt.Errorf` alone provides a message but no machine-readable metadata (error codes, field names, HTTP status mapping).
- **Inconsistent mapping**: Each HTTP handler independently decides which errors become 404 vs 400 vs 500, leading to inconsistent API responses.
- **No composability**: Without a base error type, adding cross-cutting concerns (request IDs, timestamps) requires modifying every error site.

## Solution

Build a layered error hierarchy:

1. **Error kind enumeration** using `iota` for machine-readable classification.
2. **Base `AppError`** struct implementing `error` with kind, message, and optional wrapped cause.
3. **Specialized error types** (`ValidationError`, `NotFoundError`, etc.) embedding or wrapping `AppError`.
4. **Adapter functions** that map error kinds to HTTP status codes, CLI exit codes, etc.
5. **Use `errors.Is` and `errors.As`** for matching and type assertion in callers.

## Implementation

### Project Structure

```
pkg/
  apperror/
    kinds.go          # ErrorKind enum
    app_error.go      # Base AppError type
    validation.go     # ValidationError
    not_found.go      # NotFoundError
    conflict.go       # ConflictError
    auth.go           # AuthError, ForbiddenError
    internal.go       # InternalError
    adapters.go       # HTTP/CLI/gRPC mapping
```

### Error Kind Enumeration

```go
package apperror

// ErrorKind classifies errors into categories that can be mapped
// to transport-specific codes (HTTP status, CLI exit code, etc.).
type ErrorKind int

const (
	// KindInternal represents unexpected internal failures.
	KindInternal ErrorKind = iota

	// KindValidation represents invalid input from the caller.
	KindValidation

	// KindNotFound represents a requested resource that does not exist.
	KindNotFound

	// KindConflict represents a state conflict (e.g., duplicate entry).
	KindConflict

	// KindUnauthorized represents missing or invalid authentication.
	KindUnauthorized

	// KindForbidden represents insufficient permissions.
	KindForbidden

	// KindTimeout represents an operation that exceeded its deadline.
	KindTimeout

	// KindUnavailable represents a temporarily unavailable dependency.
	KindUnavailable
)

// String returns a human-readable label for the error kind.
func (k ErrorKind) String() string {
	switch k {
	case KindInternal:
		return "internal"
	case KindValidation:
		return "validation"
	case KindNotFound:
		return "not_found"
	case KindConflict:
		return "conflict"
	case KindUnauthorized:
		return "unauthorized"
	case KindForbidden:
		return "forbidden"
	case KindTimeout:
		return "timeout"
	case KindUnavailable:
		return "unavailable"
	default:
		return "unknown"
	}
}
```

### Base AppError

```go
package apperror

import "fmt"

// AppError is the base error type for all application errors. It carries
// a Kind for classification, a human-readable Message, and an optional
// wrapped cause for error chain traversal.
type AppError struct {
	Kind    ErrorKind
	Message string
	Cause   error
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Kind, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

// Unwrap returns the wrapped cause, enabling errors.Is and errors.As
// to traverse the error chain.
func (e *AppError) Unwrap() error {
	return e.Cause
}

// Is supports sentinel matching by comparing error kinds.
func (e *AppError) Is(target error) bool {
	if t, ok := target.(*AppError); ok {
		return e.Kind == t.Kind
	}
	return false
}
```

### Sentinel Errors

```go
package apperror

// Sentinel errors for use with errors.Is(). These are "prototype" errors
// that match by Kind, not by message content.
var (
	ErrInternal     = &AppError{Kind: KindInternal}
	ErrValidation   = &AppError{Kind: KindValidation}
	ErrNotFound     = &AppError{Kind: KindNotFound}
	ErrConflict     = &AppError{Kind: KindConflict}
	ErrUnauthorized = &AppError{Kind: KindUnauthorized}
	ErrForbidden    = &AppError{Kind: KindForbidden}
	ErrTimeout      = &AppError{Kind: KindTimeout}
	ErrUnavailable  = &AppError{Kind: KindUnavailable}
)
```

### Specialized Error Types

#### ValidationError

```go
package apperror

import (
	"fmt"
	"strings"
)

// FieldError describes a validation failure on a specific field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError represents one or more field-level validation failures.
type ValidationError struct {
	AppError
	Fields []FieldError
}

// NewValidationError creates a ValidationError with one or more field errors.
func NewValidationError(fields ...FieldError) *ValidationError {
	msgs := make([]string, len(fields))
	for i, f := range fields {
		msgs[i] = fmt.Sprintf("%s: %s", f.Field, f.Message)
	}
	return &ValidationError{
		AppError: AppError{
			Kind:    KindValidation,
			Message: fmt.Sprintf("validation failed: %s", strings.Join(msgs, "; ")),
		},
		Fields: fields,
	}
}

// Error implements the error interface with field detail.
func (e *ValidationError) Error() string {
	return e.AppError.Error()
}
```

#### NotFoundError

```go
package apperror

import "fmt"

// NotFoundError indicates that a specific resource was not found.
type NotFoundError struct {
	AppError
	Resource string // e.g., "user", "document"
	ID       string // the identifier that was looked up
}

// NewNotFoundError creates a NotFoundError for a specific resource.
func NewNotFoundError(resource, id string) *NotFoundError {
	return &NotFoundError{
		AppError: AppError{
			Kind:    KindNotFound,
			Message: fmt.Sprintf("%s %q not found", resource, id),
		},
		Resource: resource,
		ID:       id,
	}
}
```

#### ConflictError

```go
package apperror

import "fmt"

// ConflictError indicates a state conflict such as a duplicate entry.
type ConflictError struct {
	AppError
	Resource string
	Field    string
	Value    string
}

// NewConflictError creates a ConflictError for a duplicate resource.
func NewConflictError(resource, field, value string) *ConflictError {
	return &ConflictError{
		AppError: AppError{
			Kind:    KindConflict,
			Message: fmt.Sprintf("%s with %s %q already exists", resource, field, value),
		},
		Resource: resource,
		Field:    field,
		Value:    value,
	}
}
```

#### AuthError

```go
package apperror

// AuthError indicates an authentication failure.
type AuthError struct {
	AppError
}

// NewAuthError creates an authentication error.
func NewAuthError(message string) *AuthError {
	return &AuthError{
		AppError: AppError{
			Kind:    KindUnauthorized,
			Message: message,
		},
	}
}

// ForbiddenError indicates an authorization (permissions) failure.
type ForbiddenError struct {
	AppError
	Action   string
	Resource string
}

// NewForbiddenError creates a permissions error.
func NewForbiddenError(action, resource string) *ForbiddenError {
	return &ForbiddenError{
		AppError: AppError{
			Kind:    KindForbidden,
			Message: fmt.Sprintf("not allowed to %s %s", action, resource),
		},
		Action:   action,
		Resource: resource,
	}
}
```

#### InternalError with Wrapping

```go
package apperror

import "fmt"

// NewInternalError wraps an unexpected error with context.
// The original error is preserved in the chain for debugging.
func NewInternalError(msg string, cause error) *AppError {
	return &AppError{
		Kind:    KindInternal,
		Message: msg,
		Cause:   cause,
	}
}

// Wrap adds context to any error while preserving the original chain.
// If the error is already an AppError, a new AppError wraps it with the
// same Kind. Otherwise, it is wrapped as KindInternal.
func Wrap(err error, msg string) *AppError {
	if err == nil {
		return nil
	}
	var appErr *AppError
	if As(err, &appErr) {
		return &AppError{
			Kind:    appErr.Kind,
			Message: msg,
			Cause:   err,
		}
	}
	return &AppError{
		Kind:    KindInternal,
		Message: msg,
		Cause:   err,
	}
}
```

### Transport Adapters

#### HTTP Adapter

```go
package apperror

import (
	"encoding/json"
	"errors"
	"net/http"
)

// HTTPErrorResponse is the JSON body returned for error responses.
type HTTPErrorResponse struct {
	Error  string       `json:"error"`
	Code   string       `json:"code"`
	Fields []FieldError `json:"fields,omitempty"`
}

// HTTPStatusCode maps an error to the appropriate HTTP status code.
func HTTPStatusCode(err error) int {
	var appErr *AppError
	if !errors.As(err, &appErr) {
		return http.StatusInternalServerError
	}

	switch appErr.Kind {
	case KindValidation:
		return http.StatusBadRequest
	case KindNotFound:
		return http.StatusNotFound
	case KindConflict:
		return http.StatusConflict
	case KindUnauthorized:
		return http.StatusUnauthorized
	case KindForbidden:
		return http.StatusForbidden
	case KindTimeout:
		return http.StatusGatewayTimeout
	case KindUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// WriteHTTPError writes a structured JSON error response.
func WriteHTTPError(w http.ResponseWriter, err error) {
	status := HTTPStatusCode(err)

	resp := HTTPErrorResponse{
		Error: err.Error(),
		Code:  "internal",
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		resp.Code = appErr.Kind.String()
	}

	var valErr *ValidationError
	if errors.As(err, &valErr) {
		resp.Fields = valErr.Fields
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}
```

#### CLI Exit Code Adapter

```go
package apperror

import "errors"

// CLIExitCode maps an error to a CLI exit code.
// Uses common conventions: 0 = success, 1 = general, 2 = usage.
func CLIExitCode(err error) int {
	if err == nil {
		return 0
	}

	var appErr *AppError
	if !errors.As(err, &appErr) {
		return 1
	}

	switch appErr.Kind {
	case KindValidation:
		return 2 // usage error
	case KindNotFound:
		return 3
	case KindUnauthorized, KindForbidden:
		return 4
	case KindConflict:
		return 5
	case KindTimeout, KindUnavailable:
		return 6
	default:
		return 1
	}
}
```

### Using errors.Is and errors.As

```go
package main

import (
	"errors"
	"fmt"
	"log"

	"yourmodule/pkg/apperror"
)

func getUser(id string) (*User, error) {
	// ... database lookup ...
	return nil, apperror.NewNotFoundError("user", id)
}

func handleRequest() {
	user, err := getUser("abc-123")
	if err != nil {
		// errors.Is: Match by kind using sentinel errors.
		if errors.Is(err, apperror.ErrNotFound) {
			fmt.Println("Resource not found, returning 404")
			return
		}
		if errors.Is(err, apperror.ErrValidation) {
			fmt.Println("Bad input, returning 400")
			return
		}

		// errors.As: Extract the concrete type for detailed info.
		var notFound *apperror.NotFoundError
		if errors.As(err, &notFound) {
			fmt.Printf("Could not find %s with ID %s\n", notFound.Resource, notFound.ID)
			return
		}

		var valErr *apperror.ValidationError
		if errors.As(err, &valErr) {
			for _, f := range valErr.Fields {
				fmt.Printf("Field %s: %s\n", f.Field, f.Message)
			}
			return
		}

		// Fallback: unexpected error.
		log.Printf("unexpected error: %v", err)
	}

	_ = user
}
```

### Error Wrapping with %w

```go
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"yourmodule/pkg/apperror"
)

func (r *UserRepo) FindByID(ctx context.Context, id string) (*User, error) {
	row := r.db.QueryRowContext(ctx, "SELECT ... WHERE id = $1", id)

	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.NewNotFoundError("user", id)
		}
		// Wrap the database error with context. The %w verb is used
		// implicitly by AppError.Cause -- Wrap preserves the chain.
		return nil, apperror.Wrap(err, fmt.Sprintf("querying user %s", id))
	}

	return &u, nil
}
```

## Benefits

### 1. Machine-Readable Classification
ErrorKind provides a finite set of categories that adapters can switch on, eliminating string parsing and ad-hoc status code decisions.

### 2. Rich Error Context
Specialized types carry structured metadata (field names, resource types, IDs) that can be serialized into API responses.

### 3. Consistent Transport Mapping
A single `HTTPStatusCode` function ensures every handler maps the same error kind to the same HTTP status, eliminating inconsistency.

### 4. Full Error Chain Support
Implementing `Unwrap()` means `errors.Is` and `errors.As` traverse the entire chain, so wrapping errors for context never loses the original classification.

## Best Practices

- **Create errors at the source**: The repository that discovers "not found" should return `NewNotFoundError`, not the HTTP handler.
- **Wrap, don't replace**: Use `apperror.Wrap(err, "context")` to add context while preserving the original error chain.
- **Match by kind first, type second**: Use `errors.Is(err, apperror.ErrNotFound)` for branching, `errors.As` only when you need the concrete type's extra fields.
- **Keep error messages for humans, codes for machines**: The `Message` field is for logs; the `Kind` field is for control flow.
- **Do not expose internal errors to clients**: The HTTP adapter should sanitize internal error messages. Return the Kind-based code and a generic message for `KindInternal`.

## Anti-Patterns

### 1. String Matching on Error Messages

```go
// BAD: Fragile, breaks if message wording changes.
if strings.Contains(err.Error(), "not found") {
    w.WriteHeader(404)
}

// GOOD: Use errors.Is with sentinel or errors.As with type.
if errors.Is(err, apperror.ErrNotFound) {
    w.WriteHeader(http.StatusNotFound)
}
```

### 2. Swallowing the Error Chain

```go
// BAD: Creates a new error, losing the original chain.
if err != nil {
    return fmt.Errorf("failed to get user: %s", err) // %s, not %w
}

// GOOD: Wrap with %w or use apperror.Wrap to preserve the chain.
if err != nil {
    return apperror.Wrap(err, "failed to get user")
}
```

### 3. Mapping Errors in Every Handler

```go
// BAD: Each handler independently maps errors to HTTP codes.
func handlerA(w http.ResponseWriter, r *http.Request) {
    // ... 20 lines of error-to-status mapping ...
}
func handlerB(w http.ResponseWriter, r *http.Request) {
    // ... same 20 lines, slightly different ...
}

// GOOD: Single adapter function used everywhere.
func handlerA(w http.ResponseWriter, r *http.Request) {
    user, err := svc.GetUser(id)
    if err != nil {
        apperror.WriteHTTPError(w, err)
        return
    }
    // ...
}
```

### 4. Defining Error Kinds Per Package

```go
// BAD: Each package defines its own error kinds.
// package users: const ErrNotFound = ...
// package products: const ErrNotFound = ...

// GOOD: Single apperror package with shared ErrorKind enum.
// All packages return apperror types.
```

## Related Patterns

- **[core-sdk-go.types-shared](./core-sdk-go.types-shared.md)**: Named domain types that appear in error metadata (e.g., UserID in NotFoundError).
- **[core-sdk-go.types-config](./core-sdk-go.types-config.md)**: Config errors (missing required fields, invalid values) use ValidationError from this pattern.

## Testing

### Testing Error Classification

```go
package apperror_test

import (
	"errors"
	"testing"

	"yourmodule/pkg/apperror"
)

func TestNotFoundError_Is(t *testing.T) {
	err := apperror.NewNotFoundError("user", "abc-123")

	if !errors.Is(err, apperror.ErrNotFound) {
		t.Error("expected NotFoundError to match ErrNotFound sentinel")
	}

	if errors.Is(err, apperror.ErrValidation) {
		t.Error("NotFoundError should not match ErrValidation sentinel")
	}
}

func TestValidationError_Fields(t *testing.T) {
	err := apperror.NewValidationError(
		apperror.FieldError{Field: "email", Message: "invalid format"},
		apperror.FieldError{Field: "name", Message: "required"},
	)

	var valErr *apperror.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatal("expected errors.As to succeed for ValidationError")
	}

	if len(valErr.Fields) != 2 {
		t.Errorf("expected 2 field errors, got %d", len(valErr.Fields))
	}
}

func TestWrap_PreservesKind(t *testing.T) {
	original := apperror.NewNotFoundError("doc", "xyz")
	wrapped := apperror.Wrap(original, "loading document")

	if !errors.Is(wrapped, apperror.ErrNotFound) {
		t.Error("wrapped error should preserve NotFound kind")
	}

	if wrapped.Message != "loading document" {
		t.Errorf("expected outer message %q, got %q", "loading document", wrapped.Message)
	}
}
```

### Testing HTTP Adapter

```go
package apperror_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"yourmodule/pkg/apperror"
)

func TestHTTPStatusCode(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"not found", apperror.NewNotFoundError("x", "1"), http.StatusNotFound},
		{"validation", apperror.NewValidationError(), http.StatusBadRequest},
		{"conflict", apperror.NewConflictError("x", "y", "z"), http.StatusConflict},
		{"unauthorized", apperror.NewAuthError("bad token"), http.StatusUnauthorized},
		{"unknown", errors.New("raw error"), http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := apperror.HTTPStatusCode(tc.err)
			if got != tc.status {
				t.Errorf("expected %d, got %d", tc.status, got)
			}
		})
	}
}

func TestWriteHTTPError_JSON(t *testing.T) {
	w := httptest.NewRecorder()
	err := apperror.NewValidationError(
		apperror.FieldError{Field: "email", Message: "required"},
	)

	apperror.WriteHTTPError(w, err)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
