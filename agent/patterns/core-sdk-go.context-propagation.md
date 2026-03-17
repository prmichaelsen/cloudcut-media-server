# Pattern: Context Propagation

**Namespace**: core-sdk-go
**Category**: Go-Specific
**Created**: 2026-03-17
**Status**: Active

---

## Overview

Defines how `context.Context` flows through every layer of a Go application -- from HTTP/gRPC handlers through service methods, repository calls, and external API invocations. Context carries deadlines, cancellation signals, and request-scoped metadata (request ID, user ID) without polluting function signatures with ambient state.

---

## Problem

Go has no implicit request scope like thread-local storage in Java or AsyncLocalStorage in Node.js. Without a consistent propagation strategy, code ends up with:

- Functions that cannot be cancelled or timed out
- Request-scoped data (trace IDs, auth info) passed via ad-hoc struct fields or globals
- Database queries and HTTP calls that hang indefinitely when the caller has already disconnected
- Inconsistent signatures where some functions accept context and others do not

---

## Solution

Adopt a strict convention: `context.Context` is always the first parameter of any function that performs I/O, may block, or participates in the request lifecycle. Use `context.WithCancel` for explicit cancellation, `context.WithTimeout` / `context.WithDeadline` for time-bounded operations, and `context.WithValue` sparingly for request-scoped metadata only.

---

## Implementation

### Context Key Types

```go
package ctxkeys

// contextKey is an unexported type to prevent collisions with keys from other packages.
type contextKey string

const (
	// RequestIDKey carries the unique request identifier.
	RequestIDKey contextKey = "request_id"
	// UserIDKey carries the authenticated user ID.
	UserIDKey contextKey = "user_id"
	// TraceIDKey carries the distributed trace identifier.
	TraceIDKey contextKey = "trace_id"
)
```

### Middleware That Enriches Context

```go
package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"myapp/ctxkeys"
)

// RequestID extracts or generates a request ID and stores it in the context.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.NewString()
		}
		ctx := context.WithValue(r.Context(), ctxkeys.RequestIDKey, reqID)
		w.Header().Set("X-Request-ID", reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserID extracts the authenticated user ID from the request (e.g., from a JWT)
// and stores it in the context.
func UserID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := extractUserID(r) // your auth logic here
		if userID != "" {
			ctx := context.WithValue(r.Context(), ctxkeys.UserIDKey, userID)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

func extractUserID(r *http.Request) string {
	// Placeholder -- implement JWT parsing, session lookup, etc.
	return r.Header.Get("X-User-ID")
}
```

### Helper Functions for Reading Context Values

```go
package ctxkeys

import "context"

// GetRequestID retrieves the request ID from the context.
// Returns an empty string if not set.
func GetRequestID(ctx context.Context) string {
	v, _ := ctx.Value(RequestIDKey).(string)
	return v
}

// GetUserID retrieves the user ID from the context.
// Returns an empty string if not set.
func GetUserID(ctx context.Context) string {
	v, _ := ctx.Value(UserIDKey).(string)
	return v
}
```

### Service Layer -- Context as First Parameter

```go
package order

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"myapp/ctxkeys"
)

type Order struct {
	ID     string
	UserID string
	Total  float64
}

type OrderRepository interface {
	FindByID(ctx context.Context, id string) (*Order, error)
	Save(ctx context.Context, o *Order) error
}

type PaymentClient interface {
	Charge(ctx context.Context, userID string, amount float64) error
}

type OrderService struct {
	repo    OrderRepository
	payment PaymentClient
	logger  *slog.Logger
}

func NewOrderService(repo OrderRepository, payment PaymentClient, logger *slog.Logger) *OrderService {
	return &OrderService{repo: repo, payment: payment, logger: logger}
}

// CreateOrder demonstrates context flowing through service -> repository -> external call.
func (s *OrderService) CreateOrder(ctx context.Context, userID string, total float64) (*Order, error) {
	reqID := ctxkeys.GetRequestID(ctx)
	s.logger.InfoContext(ctx, "creating order",
		"request_id", reqID,
		"user_id", userID,
		"total", total,
	)

	order := &Order{
		ID:     fmt.Sprintf("ord_%d", time.Now().UnixNano()),
		UserID: userID,
		Total:  total,
	}

	// Context flows to the repository -- if the caller cancels, the DB query aborts.
	if err := s.repo.Save(ctx, order); err != nil {
		return nil, fmt.Errorf("saving order: %w", err)
	}

	// Context flows to the external payment service with a tighter timeout.
	chargeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := s.payment.Charge(chargeCtx, userID, total); err != nil {
		return nil, fmt.Errorf("charging payment: %w", err)
	}

	return order, nil
}
```

### Repository Layer -- Context-Aware Database Queries

```go
package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"myapp/order"
)

type OrderRepo struct {
	db *sql.DB
}

func NewOrderRepo(db *sql.DB) *OrderRepo {
	return &OrderRepo{db: db}
}

// FindByID uses the context to make the query cancellable and deadline-aware.
func (r *OrderRepo) FindByID(ctx context.Context, id string) (*order.Order, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT id, user_id, total FROM orders WHERE id = $1", id,
	)

	var o order.Order
	if err := row.Scan(&o.ID, &o.UserID, &o.Total); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("order %s not found", id)
		}
		return nil, fmt.Errorf("querying order: %w", err)
	}
	return &o, nil
}

// Save inserts an order, respecting context cancellation.
func (r *OrderRepo) Save(ctx context.Context, o *order.Order) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO orders (id, user_id, total) VALUES ($1, $2, $3)",
		o.ID, o.UserID, o.Total,
	)
	if err != nil {
		return fmt.Errorf("inserting order: %w", err)
	}
	return nil
}
```

### Checking ctx.Done() in Long-Running Operations

```go
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// ProcessBatch demonstrates checking ctx.Done() in a long-running loop.
func ProcessBatch(ctx context.Context, items []string, logger *slog.Logger) error {
	for i, item := range items {
		// Check for cancellation before each unit of work.
		select {
		case <-ctx.Done():
			logger.WarnContext(ctx, "batch processing cancelled",
				"processed", i,
				"total", len(items),
			)
			return fmt.Errorf("cancelled after %d/%d items: %w", i, len(items), ctx.Err())
		default:
		}

		if err := processItem(ctx, item); err != nil {
			return fmt.Errorf("processing item %d: %w", i, err)
		}
	}
	return nil
}

func processItem(ctx context.Context, item string) error {
	// Simulate work that also respects context.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}
```

### Context with Cancellation -- Parent Controls Child

```go
package example

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// FetchWithFallback tries a primary source, then falls back to a secondary.
// If the primary succeeds, the secondary is cancelled immediately.
func FetchWithFallback(ctx context.Context, logger *slog.Logger) (string, error) {
	type result struct {
		data string
		err  error
	}

	primaryCtx, primaryCancel := context.WithTimeout(ctx, 2*time.Second)
	defer primaryCancel()

	ch := make(chan result, 1)
	go func() {
		data, err := fetchFromPrimary(primaryCtx)
		ch <- result{data, err}
	}()

	select {
	case r := <-ch:
		if r.err == nil {
			return r.data, nil
		}
		logger.WarnContext(ctx, "primary source failed, trying fallback", "error", r.err)
	case <-primaryCtx.Done():
		logger.WarnContext(ctx, "primary source timed out, trying fallback")
	}

	// Fallback uses the original parent context, not the timed-out one.
	return fetchFromSecondary(ctx)
}

func fetchFromPrimary(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(3 * time.Second):
		return "primary-data", nil
	}
}

func fetchFromSecondary(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(500 * time.Millisecond):
		return "secondary-data", nil
	}
}
```

---

## Benefits

1. Every function in the call chain can be cancelled by the caller -- no hanging operations
2. Deadlines propagate automatically through the entire call tree
3. Request-scoped metadata (request ID, user ID) is available at any depth without parameter drilling
4. Database drivers, HTTP clients, and gRPC clients all natively support context cancellation
5. Structured logging with `slog.InfoContext` can extract context values for consistent log correlation
6. No global or thread-local state -- everything is explicit and testable

---

## Best Practices

**Always pass context as the first parameter:**
```go
// GOOD: context is first, named ctx
func (s *Service) GetUser(ctx context.Context, id string) (*User, error)

// BAD: context buried in the middle
func (s *Service) GetUser(id string, ctx context.Context) (*User, error)
```

**Derive child contexts for tighter deadlines:**
```go
func (s *Service) CallExternalAPI(ctx context.Context) error {
    // Parent may have a 30s deadline, but this call should complete in 5s.
    apiCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    return s.client.Do(apiCtx)
}
```

**Always defer cancel():**
```go
ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
defer cancel() // Releases resources even if the timeout is not reached.
```

**Use typed keys for context values:**
```go
type contextKey string
const myKey contextKey = "my_value"
// Prevents collisions with string keys from other packages.
```

**Keep context values to request-scoped metadata only:**
```go
// GOOD: request ID, trace ID, user ID, auth token
ctx = context.WithValue(ctx, RequestIDKey, "abc-123")

// BAD: configuration, database connections, loggers
ctx = context.WithValue(ctx, "db", dbConn) // Don't do this
```

---

## Anti-Patterns

**Don't store context in a struct:**
```go
// BAD: Context is request-scoped; storing it ties the struct to one request.
type Service struct {
    ctx context.Context // NEVER do this
    db  *sql.DB
}

// GOOD: Accept context per-call.
func (s *Service) Query(ctx context.Context, q string) error {
    return s.db.QueryRowContext(ctx, q).Scan(...)
}
```

**Don't use context.WithValue as a general-purpose store:**
```go
// BAD: Passing business data through context
ctx = context.WithValue(ctx, "order", order)
ctx = context.WithValue(ctx, "items", items)
ctx = context.WithValue(ctx, "discount", 0.15)
// This is an untyped, invisible parameter list. Use explicit function arguments.
```

**Don't ignore context cancellation in loops:**
```go
// BAD: Never checks if caller has cancelled
func processBatch(ctx context.Context, items []Item) error {
    for _, item := range items {
        process(item) // Runs to completion even if ctx is cancelled
    }
    return nil
}
```

**Don't pass context.Background() when a real context is available:**
```go
// BAD: Discards the caller's deadline and cancellation
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    result, err := h.service.Query(context.Background(), "SELECT ...") // Loses request context
}

// GOOD: Propagate the request context
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    result, err := h.service.Query(r.Context(), "SELECT ...")
}
```

**Don't create context.WithCancel without calling cancel:**
```go
// BAD: Leaks resources
ctx, _ = context.WithCancel(parentCtx)

// GOOD: Always capture and defer cancel
ctx, cancel := context.WithCancel(parentCtx)
defer cancel()
```

---

## Related Patterns

- [Service Base](core-sdk-go.service-base.md) -- BaseService.Init accepts context for lifecycle management
- [Goroutine Lifecycle](core-sdk-go.goroutine-lifecycle.md) -- context cancellation for goroutine shutdown
- [Concurrent Services](core-sdk-go.concurrent-services.md) -- context propagation across multiple services

---

## Testing

### Unit Test -- Context Cancellation

```go
package order_test

import (
	"context"
	"errors"
	"log/slog"
	"io"
	"testing"
	"time"

	"myapp/order"
)

type mockRepo struct {
	delay time.Duration
}

func (m *mockRepo) FindByID(ctx context.Context, id string) (*order.Order, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(m.delay):
		return &order.Order{ID: id, UserID: "u1", Total: 42.0}, nil
	}
}

func (m *mockRepo) Save(ctx context.Context, o *order.Order) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(m.delay):
		return nil
	}
}

type mockPayment struct{}

func (m *mockPayment) Charge(ctx context.Context, userID string, amount float64) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Millisecond):
		return nil
	}
}

func TestCreateOrder_CancelledContext(t *testing.T) {
	repo := &mockRepo{delay: 1 * time.Second}
	payment := &mockPayment{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := order.NewOrderService(repo, payment, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := svc.CreateOrder(ctx, "user-1", 99.99)
	if err == nil {
		t.Fatal("expected error due to context timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got: %v", err)
	}
}

func TestCreateOrder_Success(t *testing.T) {
	repo := &mockRepo{delay: 10 * time.Millisecond}
	payment := &mockPayment{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := order.NewOrderService(repo, payment, logger)

	ctx := context.Background()
	o, err := svc.CreateOrder(ctx, "user-1", 49.99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.UserID != "user-1" {
		t.Fatalf("expected user-1, got %s", o.UserID)
	}
}
```

### Unit Test -- Context Values in Middleware

```go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"myapp/ctxkeys"
	"myapp/middleware"
)

func TestRequestIDMiddleware_GeneratesID(t *testing.T) {
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := ctxkeys.GetRequestID(r.Context())
		if reqID == "" {
			t.Fatal("expected request ID in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID response header")
	}
}

func TestRequestIDMiddleware_PreservesExisting(t *testing.T) {
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := ctxkeys.GetRequestID(r.Context())
		if reqID != "existing-id" {
			t.Fatalf("expected existing-id, got %s", reqID)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "existing-id")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
