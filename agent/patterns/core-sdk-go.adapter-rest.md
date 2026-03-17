# Pattern: REST Adapter

**Namespace**: core-sdk-go
**Category**: Adapter Layer
**Created**: 2026-03-17
**Status**: Active

---

## Overview

The REST Adapter pattern implements an HTTP server that translates REST requests into service-layer calls. It builds on the Base Adapter interface, uses Go's `net/http` stdlib as the foundation, and recommends `chi` as the router for its composable middleware and stdlib compatibility. The adapter handles request parsing, response serialization, error mapping, and middleware orchestration while keeping all business logic in the service layer.

## Problem

HTTP servers in Go often accumulate business logic in handler functions, mix error handling with response writing, and lack a consistent middleware strategy. Without a clear adapter boundary, handlers become tightly coupled to HTTP concerns, making the same logic hard to reuse in CLI or MCP adapters.

## Solution

Create a REST adapter struct that owns the `http.Server`, router, and middleware chain. Handler functions are thin: they parse the request, call a service method, map the result (or error) to an HTTP response, and write it. Service errors are translated to HTTP status codes through a centralized error mapper.

## Implementation

### Project Structure

```
internal/
  adapter/
    rest/
      rest.go          # Adapter struct, constructor, Start/Stop
      routes.go        # Route registration
      middleware.go     # Middleware definitions
      errors.go        # Error-to-HTTP mapping
      request.go       # Request parsing helpers
      response.go      # Response writing helpers
  service/
    user.go            # UserService interface + implementation
```

### Error Mapping

```go
package rest

import (
	"errors"
	"net/http"

	"myapp/internal/service"
)

// mapError translates a service-layer error to an HTTP status code.
func mapError(err error) int {
	switch {
	case errors.Is(err, service.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, service.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, service.ErrValidation):
		return http.StatusBadRequest
	case errors.Is(err, service.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, service.ErrForbidden):
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}
```

### Request and Response Helpers

```go
package rest

import (
	"encoding/json"
	"net/http"
)

// APIError is the standard error response body.
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes an error response, mapping the service error to HTTP status.
func writeError(w http.ResponseWriter, err error) {
	status := mapError(err)
	writeJSON(w, status, APIError{
		Code:    status,
		Message: err.Error(),
	})
}

// decodeJSON decodes a JSON request body into v.
func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
```

### Middleware

```go
package rest

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// Logging middleware logs each request's method, path, status, and duration.
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(ww, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.status,
				"duration", time.Since(start),
			)
		})
	}
}

// Recovery middleware recovers from panics and returns 500.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered",
						"error", rec,
						"stack", string(debug.Stack()),
					)
					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// CORS middleware adds Cross-Origin Resource Sharing headers.
func CORS(allowedOrigins string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigins)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Auth middleware validates a Bearer token using the provided check function.
func Auth(check func(token string) (string, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("Authorization")
			if len(token) < 8 || token[:7] != "Bearer " {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			userID, err := check(token[7:])
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := withUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
```

### Context Helpers for Auth

```go
package rest

import "context"

type contextKey string

const userIDKey contextKey = "userID"

func withUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}
```

### Route Registration with chi

```go
package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"myapp/internal/service"
)

func registerRoutes(r chi.Router, users service.UserService, projects service.ProjectService) {
	r.Route("/api/v1", func(r chi.Router) {
		// User routes
		r.Route("/users", func(r chi.Router) {
			r.Get("/", listUsers(users))
			r.Post("/", createUser(users))
			r.Route("/{userID}", func(r chi.Router) {
				r.Get("/", getUser(users))
				r.Put("/", updateUser(users))
				r.Delete("/", deleteUser(users))
			})
		})

		// Project routes
		r.Route("/projects", func(r chi.Router) {
			r.Get("/", listProjects(projects))
			r.Post("/", createProject(projects))
		})
	})

	// Health endpoint (no auth required)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}
```

### Handler Functions

```go
package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"myapp/internal/service"
)

type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type UserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func createUser(svc service.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateUserRequest
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, APIError{
				Code:    http.StatusBadRequest,
				Message: "invalid request body",
			})
			return
		}

		user, err := svc.Create(r.Context(), service.CreateUserInput{
			Name:  req.Name,
			Email: req.Email,
		})
		if err != nil {
			writeError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, UserResponse{
			ID:    user.ID,
			Name:  user.Name,
			Email: user.Email,
		})
	}
}

func getUser(svc service.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "userID")

		user, err := svc.Get(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, UserResponse{
			ID:    user.ID,
			Name:  user.Name,
			Email: user.Email,
		})
	}
}

func listUsers(svc service.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := svc.List(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}

		resp := make([]UserResponse, len(users))
		for i, u := range users {
			resp[i] = UserResponse{ID: u.ID, Name: u.Name, Email: u.Email}
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func updateUser(svc service.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "userID")

		var req CreateUserRequest
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, APIError{
				Code:    http.StatusBadRequest,
				Message: "invalid request body",
			})
			return
		}

		user, err := svc.Update(r.Context(), id, service.UpdateUserInput{
			Name:  req.Name,
			Email: req.Email,
		})
		if err != nil {
			writeError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, UserResponse{
			ID:    user.ID,
			Name:  user.Name,
			Email: user.Email,
		})
	}
}

func deleteUser(svc service.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "userID")

		if err := svc.Delete(r.Context(), id); err != nil {
			writeError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

### Complete REST Adapter (Start/Stop)

```go
package rest

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"myapp/internal/adapter"
	"myapp/internal/service"
)

// Adapter is the HTTP REST adapter.
type Adapter struct {
	adapter.BaseAdapter
	server *http.Server
	addr   string
}

// New creates a fully wired REST adapter.
func New(
	addr string,
	logger *slog.Logger,
	users service.UserService,
	projects service.ProjectService,
	authCheck func(token string) (string, error),
) *Adapter {
	r := chi.NewRouter()

	// Global middleware chain (order matters).
	r.Use(Recovery(logger))
	r.Use(Logging(logger))
	r.Use(CORS("*"))

	// Public routes (health, etc.) are registered first.
	registerRoutes(r, users, projects)

	// Wrap authenticated routes.
	r.Group(func(r chi.Router) {
		r.Use(Auth(authCheck))
		// Protected routes go here if needed.
	})

	return &Adapter{
		BaseAdapter: adapter.NewBaseAdapter("rest", logger),
		server: &http.Server{
			Addr:         addr,
			Handler:      r,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		addr: addr,
	}
}

// Start begins serving HTTP traffic. Blocks until ctx is cancelled.
func (a *Adapter) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", a.addr)
	if err != nil {
		a.SetHealth(false, err.Error())
		return err
	}

	a.SetHealth(true, "listening on "+a.addr)
	a.Logger.Info("rest adapter started", "addr", a.addr)

	errCh := make(chan error, 1)
	go func() { errCh <- a.server.Serve(ln) }()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			a.SetHealth(false, err.Error())
			return err
		}
		return nil
	}
}

// Stop performs graceful HTTP shutdown.
func (a *Adapter) Stop(ctx context.Context) error {
	a.Logger.Info("rest adapter stopping")
	a.SetHealth(false, "shutting down")
	return a.server.Shutdown(ctx)
}
```

## Benefits

1. **Stdlib Foundation**: Uses `net/http` directly, so any Go HTTP library or middleware is compatible.
2. **Clean Handler Functions**: Each handler is a factory function returning `http.HandlerFunc`, making dependencies explicit and testable.
3. **Centralized Error Mapping**: Service errors map to HTTP status codes in one place, ensuring consistent API error responses.
4. **Composable Middleware**: The `func(http.Handler) http.Handler` pattern is the Go standard, and chi supports it natively.
5. **Graceful Shutdown**: The adapter cleanly drains in-flight requests before exiting.

## Best Practices

- Return handler factories (`func(svc Service) http.HandlerFunc`) rather than methods on a large handler struct. This keeps dependencies explicit.
- Use `chi.URLParam` for path parameters; avoid parsing `r.URL.Path` manually.
- Always call `defer r.Body.Close()` or use `http.MaxBytesReader` to limit request body size.
- Define request/response types as separate structs, even if they mirror domain types. This decouples the API shape from the internal model.
- Set `DisallowUnknownFields()` on the JSON decoder in strict APIs to reject unexpected fields.

## Anti-Patterns

### Leaking Domain Types to the API

**Bad**: Returning service-layer structs directly as JSON. If you add an internal field later, it leaks to clients.

```go
// Bad: exposing internal struct
writeJSON(w, http.StatusOK, user)
```

**Good**: Map to a dedicated response type.

```go
// Good: explicit API contract
writeJSON(w, http.StatusOK, UserResponse{
    ID:   user.ID,
    Name: user.Name,
})
```

### Giant Handler Structs

**Bad**: A single `Handlers` struct with 30 methods and 10 service fields.

**Good**: Handler factories that close over only the services they need.

### Inline Error Status Codes

**Bad**: Scattered `http.StatusNotFound` and `http.StatusBadRequest` checks in every handler.

**Good**: A single `writeError(w, err)` call that uses the centralized error mapper.

## Related Patterns

- **[adapter-base](./core-sdk-go.adapter-base.md)**: The lifecycle interface this adapter implements.
- **[adapter-mcp](./core-sdk-go.adapter-mcp.md)**: Alternative transport for AI tool access.
- **[adapter-client](./core-sdk-go.adapter-client.md)**: The client SDK that calls this REST API.

## Testing

### Handler Unit Tests with httptest

```go
package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"myapp/internal/adapter/rest"
	"myapp/internal/service"
)

type mockUserService struct {
	createFn func(ctx context.Context, input service.CreateUserInput) (*service.User, error)
}

func (m *mockUserService) Create(ctx context.Context, input service.CreateUserInput) (*service.User, error) {
	return m.createFn(ctx, input)
}

// ... other methods

func TestCreateUser(t *testing.T) {
	mock := &mockUserService{
		createFn: func(_ context.Context, input service.CreateUserInput) (*service.User, error) {
			return &service.User{
				ID:    "usr_123",
				Name:  input.Name,
				Email: input.Email,
			}, nil
		},
	}

	handler := rest.CreateUserHandler(mock)

	body, _ := json.Marshal(rest.CreateUserRequest{
		Name:  "Alice",
		Email: "alice@example.com",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	var resp rest.UserResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Name != "Alice" {
		t.Fatalf("expected name Alice, got %s", resp.Name)
	}
}
```

### Integration Test with the Full Router

```go
package rest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"log/slog"
	"myapp/internal/adapter/rest"
)

func TestHealthEndpoint(t *testing.T) {
	adapter := rest.New(":0", slog.Default(), mockUsers, mockProjects, noopAuth)
	ts := httptest.NewServer(adapter.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
