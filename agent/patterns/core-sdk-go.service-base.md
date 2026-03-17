# Pattern: Service Base

**Namespace**: core-sdk-go
**Category**: Service Architecture
**Created**: 2026-03-17
**Status**: Active

---

## Overview

Defines the foundational service struct in Go using composition (embedded structs) instead of class inheritance. Every service embeds `BaseService` to gain lifecycle management, state tracking, configuration access, and health checks.

---

## Problem

Go has no abstract classes or inheritance. Services need consistent lifecycle management (initialization, shutdown), state tracking (created, running, stopped, errored), configuration, and health reporting without duplicating boilerplate across every service.

---

## Solution

Use an embedded `BaseService` struct that provides lifecycle methods (`Init`, `Close`), a state machine (`ServiceState`), typed configuration via generics, and a health check interface. Concrete services embed `BaseService` and implement domain-specific logic. Interfaces define contracts at the consumer side.

---

## Implementation

### Go Implementation

```go
package service

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ServiceState represents the lifecycle state of a service.
type ServiceState int

const (
	StateCreated ServiceState = iota
	StateInitializing
	StateRunning
	StateStopping
	StateStopped
	StateErrored
)

func (s ServiceState) String() string {
	switch s {
	case StateCreated:
		return "created"
	case StateInitializing:
		return "initializing"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateStopped:
		return "stopped"
	case StateErrored:
		return "errored"
	default:
		return "unknown"
	}
}

// HealthStatus reports the health of a service.
type HealthStatus struct {
	Healthy   bool
	Message   string
	CheckedAt time.Time
}

// BaseService provides lifecycle management, state tracking, and health checks.
// Embed this in concrete service structs.
type BaseService struct {
	name  string
	state ServiceState
	mu    sync.RWMutex
	err   error
}

// NewBaseService creates a BaseService with the given name.
func NewBaseService(name string) BaseService {
	return BaseService{
		name:  name,
		state: StateCreated,
	}
}

// Name returns the service name.
func (b *BaseService) Name() string {
	return b.name
}

// State returns the current service state.
func (b *BaseService) State() ServiceState {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

// SetState transitions the service to a new state.
func (b *BaseService) SetState(state ServiceState) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = state
}

// SetError records an error and transitions to the errored state.
func (b *BaseService) SetError(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.err = err
	b.state = StateErrored
}

// Err returns the last recorded error, if any.
func (b *BaseService) Err() error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.err
}

// Health returns a basic health status derived from the service state.
func (b *BaseService) Health() HealthStatus {
	b.mu.RLock()
	defer b.mu.RUnlock()
	healthy := b.state == StateRunning
	msg := fmt.Sprintf("service %s is %s", b.name, b.state)
	if b.err != nil {
		msg = fmt.Sprintf("service %s error: %v", b.name, b.err)
	}
	return HealthStatus{
		Healthy:   healthy,
		Message:   msg,
		CheckedAt: time.Now(),
	}
}
```

### Example Usage

```go
package user

import (
	"context"
	"fmt"
	"log/slog"

	"myapp/service"
)

// UserServiceConfig holds configuration for the user service.
type UserServiceConfig struct {
	MaxUsers    int
	DefaultRole string
}

// UserService manages user operations. It embeds BaseService for lifecycle.
type UserService struct {
	service.BaseService
	config UserServiceConfig
	logger *slog.Logger
	// repo would be injected via constructor
	repo UserRepository
}

// UserRepository defines what the UserService needs from a data source.
type UserRepository interface {
	FindByID(ctx context.Context, id string) (*User, error)
	Save(ctx context.Context, u *User) error
}

type User struct {
	ID   string
	Name string
	Role string
}

// NewUserService constructs a UserService with all dependencies.
func NewUserService(cfg UserServiceConfig, repo UserRepository, logger *slog.Logger) *UserService {
	return &UserService{
		BaseService: service.NewBaseService("user-service"),
		config:      cfg,
		logger:      logger,
		repo:        repo,
	}
}

// Init initializes the user service. It respects context cancellation.
func (s *UserService) Init(ctx context.Context) error {
	s.SetState(service.StateInitializing)
	s.logger.InfoContext(ctx, "initializing user service",
		"maxUsers", s.config.MaxUsers,
	)

	// Perform any setup (migrations, cache warming, etc.)
	select {
	case <-ctx.Done():
		s.SetError(ctx.Err())
		return ctx.Err()
	default:
	}

	s.SetState(service.StateRunning)
	return nil
}

// Close shuts down the user service gracefully.
func (s *UserService) Close() error {
	s.SetState(service.StateStopping)
	s.logger.Info("shutting down user service")
	// Clean up resources here
	s.SetState(service.StateStopped)
	return nil
}

// GetUser retrieves a user by ID.
func (s *UserService) GetUser(ctx context.Context, id string) (*User, error) {
	if s.State() != service.StateRunning {
		return nil, fmt.Errorf("user service is not running (state: %s)", s.State())
	}
	return s.repo.FindByID(ctx, id)
}
```

---

## Benefits

1. Consistent lifecycle across all services via embedded BaseService
2. Thread-safe state management with sync.RWMutex
3. Context-aware initialization supports timeouts and cancellation
4. No framework dependency -- pure Go composition
5. Health checks derived from state without extra infrastructure
6. Easy to test -- embed in test doubles or mock individual methods

---

## Best Practices

**Do use context.Context for Init:**
```go
func (s *MyService) Init(ctx context.Context) error {
    // Respect cancellation
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }
    // ... init logic
    return nil
}
```

**Do check state before processing requests:**
```go
func (s *MyService) DoWork(ctx context.Context) error {
    if s.State() != service.StateRunning {
        return fmt.Errorf("service not running: %s", s.State())
    }
    // ... proceed
    return nil
}
```

**Do return errors from Close for logging:**
```go
func (s *MyService) Close() error {
    s.SetState(service.StateStopping)
    if err := s.db.Close(); err != nil {
        s.SetError(err)
        return fmt.Errorf("closing db: %w", err)
    }
    s.SetState(service.StateStopped)
    return nil
}
```

---

## Anti-Patterns

**Don't use interface embedding to simulate inheritance:**
```go
// BAD: Don't try to simulate abstract methods via interface fields
type BaseService struct {
    OnInit func(ctx context.Context) error // Not idiomatic
}
```

**Don't ignore context in Init:**
```go
// BAD: Ignoring context means no timeout/cancellation support
func (s *MyService) Init() error { // Missing context.Context
    time.Sleep(30 * time.Second) // Blocks forever, no way to cancel
    return nil
}
```

**Don't embed by pointer:**
```go
// BAD: Pointer embedding leads to nil dereference if forgotten
type MyService struct {
    *service.BaseService // Dangerous -- nil if not initialized
}

// GOOD: Value embedding is zero-value safe
type MyService struct {
    service.BaseService
}
```

---

## Related Patterns

- [Service Interface](core-sdk-go.service-interface.md) -- defining consumer-side interfaces
- [Service Container](core-sdk-go.service-container.md) -- wiring services together
- [Service Error Handling](core-sdk-go.service-error-handling.md) -- structured error returns
- [Service Logging](core-sdk-go.service-logging.md) -- structured logging with slog

---

## Testing

### Unit Test Example

```go
package user_test

import (
	"context"
	"log/slog"
	"testing"

	"myapp/service"
	"myapp/user"
)

// stubRepo is a test double for UserRepository.
type stubRepo struct {
	users map[string]*user.User
}

func (r *stubRepo) FindByID(_ context.Context, id string) (*user.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, fmt.Errorf("user %s not found", id)
	}
	return u, nil
}

func (r *stubRepo) Save(_ context.Context, u *user.User) error {
	r.users[u.ID] = u
	return nil
}

func TestUserService_Lifecycle(t *testing.T) {
	repo := &stubRepo{users: make(map[string]*user.User)}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := user.UserServiceConfig{MaxUsers: 100, DefaultRole: "viewer"}

	svc := user.NewUserService(cfg, repo, logger)

	if svc.State() != service.StateCreated {
		t.Fatalf("expected state created, got %s", svc.State())
	}

	ctx := context.Background()
	if err := svc.Init(ctx); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if svc.State() != service.StateRunning {
		t.Fatalf("expected state running, got %s", svc.State())
	}

	health := svc.Health()
	if !health.Healthy {
		t.Fatalf("expected healthy, got: %s", health.Message)
	}

	if err := svc.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if svc.State() != service.StateStopped {
		t.Fatalf("expected state stopped, got %s", svc.State())
	}
}

func TestUserService_RejectsRequestsWhenNotRunning(t *testing.T) {
	repo := &stubRepo{users: make(map[string]*user.User)}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := user.UserServiceConfig{MaxUsers: 100, DefaultRole: "viewer"}

	svc := user.NewUserService(cfg, repo, logger)

	// Service not yet initialized -- should reject
	_, err := svc.GetUser(context.Background(), "123")
	if err == nil {
		t.Fatal("expected error when service is not running")
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
