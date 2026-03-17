# Pattern: Service Logging

**Namespace**: core-sdk-go
**Category**: Observability
**Created**: 2026-03-17
**Status**: Active

---

## Overview

Structured logging using Go 1.21's `log/slog` standard library package. Services receive a `*slog.Logger` via constructor injection. Child loggers carry service context via `slog.With()`. Context-aware logging integrates with request tracing. JSON output in production, text output in development.

---

## Problem

Unstructured log messages (`log.Printf("user %s not found", id)`) are difficult to search, filter, and aggregate in production systems. Without a consistent logging approach, services produce inconsistent formats, miss important context (request IDs, service names), and make debugging across distributed systems painful.

---

## Solution

Use `log/slog` (standard library since Go 1.21) for structured, leveled logging. Pass `*slog.Logger` through service constructors. Use `slog.With()` to create child loggers that carry service-level context. Use `slog.InfoContext()` and friends for request-scoped context propagation. Choose `JSONHandler` for production and `TextHandler` for development.

---

## Implementation

### Go Implementation

#### Logger Setup

```go
package logging

import (
	"io"
	"log/slog"
	"os"
)

// NewLogger creates a configured slog.Logger.
func NewLogger(level slog.Level, format string, w io.Writer) *slog.Logger {
	if w == nil {
		w = os.Stdout
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
	}

	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(w, opts)
	default:
		handler = slog.NewTextHandler(w, opts)
	}

	return slog.New(handler)
}

// NewProductionLogger creates a JSON logger for production.
func NewProductionLogger() *slog.Logger {
	return NewLogger(slog.LevelInfo, "json", os.Stdout)
}

// NewDevelopmentLogger creates a text logger for local development.
func NewDevelopmentLogger() *slog.Logger {
	return NewLogger(slog.LevelDebug, "text", os.Stderr)
}
```

#### Service with Injected Logger

```go
package user

import (
	"context"
	"log/slog"
)

// UserService receives a logger through its constructor.
type UserService struct {
	repo   UserRepository
	logger *slog.Logger
}

// NewUserService constructs a UserService with a child logger.
func NewUserService(repo UserRepository, logger *slog.Logger) *UserService {
	return &UserService{
		repo: repo,
		// slog.With creates a child logger that includes "service" in every message
		logger: logger.With("service", "user"),
	}
}

func (s *UserService) GetByID(ctx context.Context, id string) (*User, error) {
	s.logger.DebugContext(ctx, "fetching user", "userID", id)

	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to fetch user",
			"userID", id,
			"error", err,
		)
		return nil, err
	}

	s.logger.InfoContext(ctx, "user fetched successfully",
		"userID", u.ID,
		"userName", u.Name,
	)
	return u, nil
}

func (s *UserService) Create(ctx context.Context, name, email string) (*User, error) {
	s.logger.InfoContext(ctx, "creating user",
		"name", name,
		"email", email,
	)

	u := &User{Name: name, Email: email}
	if err := s.repo.Save(ctx, u); err != nil {
		s.logger.ErrorContext(ctx, "failed to create user",
			"name", name,
			"error", err,
		)
		return nil, err
	}

	s.logger.InfoContext(ctx, "user created",
		"userID", u.ID,
	)
	return u, nil
}
```

#### Context-Aware Logging with Request IDs

```go
package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

const requestIDKey contextKey = "requestID"

// RequestID extracts the request ID from context.
func RequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// WithRequestID adds a request ID to the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDMiddleware injects a request ID into the context.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.New().String()
		}
		ctx := WithRequestID(r.Context(), id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ContextHandler is a slog.Handler that extracts values from context.
type ContextHandler struct {
	inner slog.Handler
}

func NewContextHandler(inner slog.Handler) *ContextHandler {
	return &ContextHandler{inner: inner}
}

func (h *ContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	// Automatically add request ID from context to every log entry
	if reqID := RequestID(ctx); reqID != "" {
		r.AddAttrs(slog.String("requestID", reqID))
	}
	return h.inner.Handle(ctx, r)
}

func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{inner: h.inner.WithGroup(name)}
}
```

#### Log Levels

```go
package logging

import "log/slog"

// Standard slog levels:
//   slog.LevelDebug  = -4  // Verbose diagnostic info
//   slog.LevelInfo   =  0  // Normal operational messages
//   slog.LevelWarn   =  4  // Something unexpected but recoverable
//   slog.LevelError  =  8  // Something failed

// Custom levels can be defined as needed:
const (
	LevelTrace slog.Level = -8 // Even more verbose than Debug
	LevelFatal slog.Level = 12 // Unrecoverable, program should exit
)

// ParseLevel converts a string to a slog.Level.
func ParseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
```

### Example Usage

```go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"myapp/logging"
	"myapp/middleware"
	"myapp/user"
)

func main() {
	// Create base logger
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	contextHandler := middleware.NewContextHandler(jsonHandler)
	logger := slog.New(contextHandler)

	// Set as default for any code using slog.Info() etc.
	slog.SetDefault(logger)

	// Inject logger into services
	userSvc := user.NewUserService(userRepo, logger)

	// In an HTTP handler
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		// Context carries the request ID automatically
		ctx := r.Context()
		id := r.PathValue("id")

		u, err := userSvc.GetByID(ctx, id)
		if err != nil {
			// The error log inside GetByID already includes requestID via context
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		// ... respond with user
		_ = u
	})

	handler := middleware.RequestIDMiddleware(mux)
	http.ListenAndServe(":8080", handler)
}

// Output (JSON, production):
// {"time":"2026-03-17T10:30:00Z","level":"INFO","msg":"user fetched successfully","service":"user","userID":"abc-123","userName":"Alice","requestID":"req-uuid-here"}
```

#### Passing Logger Through Container

```go
package app

import (
	"log/slog"

	"myapp/order"
	"myapp/user"
)

func NewContainer(cfg Config) *Container {
	logger := logging.NewProductionLogger()

	return &Container{
		UserSvc:  user.NewUserService(repo, logger),                    // gets logger.With("service","user")
		OrderSvc: order.NewOrderService(repo, userSvc, logger),         // gets logger.With("service","order")
	}
}
```

---

## Benefits

1. Structured key-value pairs are searchable and machine-parsable
2. Standard library -- no third-party dependency for core logging
3. Child loggers via `slog.With()` add context without repetition
4. Context-aware methods (`InfoContext`, `ErrorContext`) integrate with request tracing
5. Handler interface is extensible -- swap JSON/text or add custom enrichment
6. Log levels filter noise in production while allowing verbose output in development
7. Constructor injection makes the logger dependency explicit and testable

---

## Best Practices

**Do inject loggers via constructors:**
```go
func NewService(repo Repository, logger *slog.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger.With("service", "myservice"),
	}
}
```

**Do use context-aware logging methods:**
```go
// GOOD: request context flows through to the handler
s.logger.InfoContext(ctx, "operation completed", "duration", elapsed)

// Acceptable for background/startup messages without request context
s.logger.Info("service initialized", "port", cfg.Port)
```

**Do use structured key-value pairs, not formatted strings:**
```go
// GOOD: structured, searchable
s.logger.Info("user created", "userID", u.ID, "email", u.Email)

// BAD: unstructured, hard to parse
s.logger.Info(fmt.Sprintf("user %s created with email %s", u.ID, u.Email))
```

**Do use slog.Group for related attributes:**
```go
s.logger.Info("request handled",
	slog.Group("request",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
	),
	slog.Group("response",
		slog.Int("status", status),
		slog.Duration("duration", elapsed),
	),
)
// Output: {"request":{"method":"GET","path":"/users"},"response":{"status":200,"duration":"12ms"}}
```

---

## Anti-Patterns

**Don't use the global default logger without setting it up:**
```go
// BAD: uses the default logger which may not be configured
slog.Info("something happened")

// GOOD: use an injected logger or set the default explicitly first
slog.SetDefault(myConfiguredLogger)
```

**Don't log and return the same error (log at the boundary):**
```go
// BAD: error gets logged multiple times as it propagates
func (s *Service) GetUser(ctx context.Context, id string) (*User, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to find user", "error", err) // Logged here
		return nil, fmt.Errorf("getting user: %w", err)     // AND logged by caller
	}
	return u, nil
}

// GOOD: return the error, let the boundary (HTTP handler) log it
func (s *Service) GetUser(ctx context.Context, id string) (*User, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting user %s: %w", id, err)
	}
	return u, nil
}
```

**Don't log sensitive data:**
```go
// BAD: passwords, tokens, PII in logs
s.logger.Info("user login", "password", password, "ssn", ssn)

// GOOD: log identifiers, not secrets
s.logger.Info("user login", "userID", userID)
```

**Don't create a new logger per request:**
```go
// BAD: allocates a new logger on every call
func (s *Service) Handle(ctx context.Context) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("handling request")
}

// GOOD: use the injected logger, add per-request context via slog.With
func (s *Service) Handle(ctx context.Context) {
	s.logger.InfoContext(ctx, "handling request")
}
```

---

## Related Patterns

- [Service Base](core-sdk-go.service-base.md) -- services embed BaseService and carry a logger
- [Service Container](core-sdk-go.service-container.md) -- container creates and distributes loggers
- [Service Error Handling](core-sdk-go.service-error-handling.md) -- logging errors with severity context

---

## Alternatives

While `slog` is recommended for its standard library stability and zero-dependency nature, two popular alternatives exist:

- **[zap](https://github.com/uber-go/zap)**: Higher performance (zero-allocation), richer encoder options. Use when logging throughput is critical (>100k logs/sec).
- **[zerolog](https://github.com/rs/zerolog)**: Similar zero-allocation design, fluent API. Use when you prefer method chaining (`log.Info().Str("key", "val").Msg("done")`).

Both can be wrapped behind `slog.Handler` to maintain a unified interface.

---

## Testing

### Unit Test Example

```go
package user_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"myapp/user"
)

func TestUserService_LogsOnCreate(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	repo := &stubRepo{users: make(map[string]*user.User)}
	svc := user.NewUserService(repo, logger)

	ctx := context.Background()
	_, err := svc.Create(ctx, "Alice", "alice@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify structured log output
	output := buf.String()
	if !strings.Contains(output, `"msg":"creating user"`) {
		t.Errorf("expected 'creating user' log, got:\n%s", output)
	}
	if !strings.Contains(output, `"service":"user"`) {
		t.Errorf("expected service attribute in log, got:\n%s", output)
	}
}

func TestUserService_LogsErrorOnFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	repo := &failingRepo{err: fmt.Errorf("connection refused")}
	svc := user.NewUserService(repo, logger)

	ctx := context.Background()
	_, err := svc.Create(ctx, "Bob", "bob@example.com")
	if err == nil {
		t.Fatal("expected error")
	}

	output := buf.String()
	if !strings.Contains(output, `"level":"ERROR"`) {
		t.Errorf("expected ERROR level log, got:\n%s", output)
	}
	if !strings.Contains(output, "connection refused") {
		t.Errorf("expected error message in log, got:\n%s", output)
	}
}

func TestContextHandler_AddsRequestID(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, nil)
	handler := middleware.NewContextHandler(inner)
	logger := slog.New(handler)

	ctx := middleware.WithRequestID(context.Background(), "req-abc-123")
	logger.InfoContext(ctx, "test message")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}

	if entry["requestID"] != "req-abc-123" {
		t.Errorf("expected requestID 'req-abc-123', got %v", entry["requestID"])
	}
}

// Discard logger for tests that don't care about log output
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
