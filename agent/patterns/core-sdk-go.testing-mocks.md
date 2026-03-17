# Pattern: Mocking via Interfaces

**Namespace**: core-sdk-go
**Category**: Testing
**Created**: 2026-03-17
**Status**: Active

---

## Overview

Go's interface-based design makes mocking natural: define a small interface, inject it as a dependency, and swap in a mock during tests. This pattern covers three approaches -- hand-written mocks (preferred for small interfaces), gomock/mockgen for larger interfaces, and testify/mock as an alternative -- with guidance on when to use each.

## Problem

Testing code that depends on databases, HTTP APIs, or other external services requires isolating the unit under test. Without interfaces, you end up testing against real services (slow, flaky) or resorting to monkey-patching (brittle, non-idiomatic). Go lacks the reflection-based mocking common in languages with class hierarchies.

## Solution

Define narrow interfaces at the consumer site (not the provider). Inject these interfaces as dependencies. In tests, provide a mock implementation that records calls and returns configured responses. Prefer hand-written mocks for interfaces with 1-3 methods; use code generation for larger interfaces.

## Implementation

### Interface-Based Dependency Injection

```go
package order

import "context"

// UserFetcher is defined where it's consumed, not where it's implemented.
// This follows Go's implicit interface satisfaction.
type UserFetcher interface {
	GetUser(ctx context.Context, id string) (*User, error)
}

type Service struct {
	users UserFetcher
}

func NewService(users UserFetcher) *Service {
	return &Service{users: users}
}

func (s *Service) PlaceOrder(ctx context.Context, userID string, items []Item) (*Order, error) {
	user, err := s.users.GetUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fetch user: %w", err)
	}
	if !user.Active {
		return nil, ErrInactiveUser
	}
	// ... create order
	return &Order{UserID: userID, Items: items}, nil
}
```

### Approach 1: Hand-Written Mock (Preferred for Small Interfaces)

Use function fields for maximum flexibility. Each test can configure exactly the behavior it needs:

```go
package order_test

import (
	"context"
	"errors"
	"testing"

	"github.com/example/project/order"
)

// mockUserFetcher implements order.UserFetcher with configurable behavior.
type mockUserFetcher struct {
	GetUserFunc func(ctx context.Context, id string) (*order.User, error)
}

func (m *mockUserFetcher) GetUser(ctx context.Context, id string) (*order.User, error) {
	return m.GetUserFunc(ctx, id)
}

func TestPlaceOrder_ActiveUser(t *testing.T) {
	mock := &mockUserFetcher{
		GetUserFunc: func(ctx context.Context, id string) (*order.User, error) {
			return &order.User{ID: id, Name: "Alice", Active: true}, nil
		},
	}

	svc := order.NewService(mock)
	o, err := svc.PlaceOrder(context.Background(), "user-1", []order.Item{{SKU: "ABC"}})
	if err != nil {
		t.Fatalf("PlaceOrder() error: %v", err)
	}
	if o.UserID != "user-1" {
		t.Errorf("order UserID = %q, want %q", o.UserID, "user-1")
	}
}

func TestPlaceOrder_InactiveUser(t *testing.T) {
	mock := &mockUserFetcher{
		GetUserFunc: func(ctx context.Context, id string) (*order.User, error) {
			return &order.User{ID: id, Active: false}, nil
		},
	}

	svc := order.NewService(mock)
	_, err := svc.PlaceOrder(context.Background(), "user-2", []order.Item{{SKU: "ABC"}})
	if !errors.Is(err, order.ErrInactiveUser) {
		t.Errorf("PlaceOrder() error = %v, want %v", err, order.ErrInactiveUser)
	}
}

func TestPlaceOrder_FetchError(t *testing.T) {
	mock := &mockUserFetcher{
		GetUserFunc: func(ctx context.Context, id string) (*order.User, error) {
			return nil, errors.New("connection refused")
		},
	}

	svc := order.NewService(mock)
	_, err := svc.PlaceOrder(context.Background(), "user-3", nil)
	if err == nil {
		t.Fatal("PlaceOrder() expected error, got nil")
	}
}
```

### Hand-Written Mock with Call Recording

When you need to verify the mock was called correctly:

```go
package order_test

import (
	"context"
	"sync"

	"github.com/example/project/order"
)

type spyUserFetcher struct {
	mu    sync.Mutex
	calls []string

	GetUserFunc func(ctx context.Context, id string) (*order.User, error)
}

func (s *spyUserFetcher) GetUser(ctx context.Context, id string) (*order.User, error) {
	s.mu.Lock()
	s.calls = append(s.calls, id)
	s.mu.Unlock()
	return s.GetUserFunc(ctx, id)
}

func (s *spyUserFetcher) CallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func (s *spyUserFetcher) CalledWith() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make([]string, len(s.calls))
	copy(copied, s.calls)
	return copied
}
```

### Approach 2: gomock / mockgen (For Larger Interfaces)

Install mockgen:
```bash
go install go.uber.org/mock/mockgen@latest
```

Generate mocks:
```bash
mockgen -source=repository.go -destination=mocks/mock_repository.go -package=mocks
```

Source interface:
```go
package store

import "context"

//go:generate mockgen -source=repository.go -destination=mocks/mock_repository.go -package=mocks

type Repository interface {
	GetByID(ctx context.Context, id string) (*Entity, error)
	List(ctx context.Context, filter Filter) ([]*Entity, error)
	Create(ctx context.Context, e *Entity) error
	Update(ctx context.Context, e *Entity) error
	Delete(ctx context.Context, id string) error
}
```

Using the generated mock:
```go
package store_test

import (
	"context"
	"testing"

	"github.com/example/project/store"
	"github.com/example/project/store/mocks"
	"go.uber.org/mock/gomock"
)

func TestService_GetEntity(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockRepo := mocks.NewMockRepository(ctrl)
	mockRepo.EXPECT().
		GetByID(gomock.Any(), "entity-1").
		Return(&store.Entity{ID: "entity-1", Name: "Test"}, nil).
		Times(1)

	svc := store.NewService(mockRepo)
	entity, err := svc.GetEntity(context.Background(), "entity-1")
	if err != nil {
		t.Fatalf("GetEntity() error: %v", err)
	}
	if entity.Name != "Test" {
		t.Errorf("entity.Name = %q, want %q", entity.Name, "Test")
	}
}
```

### Approach 3: testify/mock

```go
package store_test

import (
	"context"
	"testing"

	"github.com/example/project/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) GetByID(ctx context.Context, id string) (*store.Entity, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.Entity), args.Error(1)
}

func (m *mockRepository) Create(ctx context.Context, e *store.Entity) error {
	args := m.Called(ctx, e)
	return args.Error(0)
}

// Implement other interface methods similarly...

func TestService_GetEntity_Testify(t *testing.T) {
	repo := new(mockRepository)
	repo.On("GetByID", mock.Anything, "entity-1").
		Return(&store.Entity{ID: "entity-1", Name: "Test"}, nil)

	svc := store.NewService(repo)
	entity, err := svc.GetEntity(context.Background(), "entity-1")

	assert.NoError(t, err)
	assert.Equal(t, "Test", entity.Name)
	repo.AssertExpectations(t)
}
```

### Comparison of Approaches

| Aspect | Hand-Written | gomock | testify/mock |
|--------|-------------|--------|-------------|
| Setup effort | Low for small interfaces | Code generation step | Medium boilerplate |
| Interface size sweet spot | 1-3 methods | 4+ methods | 3+ methods |
| Type safety | Full (compile-time) | Full (generated code) | Partial (runtime type assertions) |
| Expectation verification | Manual (spy pattern) | Built-in (EXPECT) | Built-in (AssertExpectations) |
| Dependencies | None | go.uber.org/mock | github.com/stretchr/testify |
| Learning curve | Minimal | Moderate | Low-moderate |
| Flexibility | Maximum | High | High |

## Benefits

1. **Interface-based DI is idiomatic Go** - No framework needed; implicit interface satisfaction makes this natural.
2. **Hand-written mocks are transparent** - No magic; the mock is just a struct you can read.
3. **Compile-time safety** - If the interface changes, the mock fails to compile immediately.
4. **Flexible per-test behavior** - Function fields let each test configure exactly what it needs.
5. **Small interfaces are easy to mock** - Go's convention of small, focused interfaces (1-3 methods) means mocks stay simple.

## Best Practices

- Define interfaces at the consumer, not the provider. A `UserFetcher` in the `order` package is better than a `UserService` interface in the `user` package.
- Keep interfaces small. If you need to mock 10 methods, the interface is probably too wide.
- Prefer hand-written mocks for interfaces with 1-3 methods. The overhead of gomock is not worth it for trivial interfaces.
- Use `//go:generate` comments next to interfaces to keep generated mocks up to date.
- Place generated mocks in a `mocks/` subdirectory or alongside test files.
- Use `gomock.Any()` for context parameters rather than matching specific context values.
- Always call `ctrl.Finish()` or use `gomock.NewController(t)` (which calls Finish automatically via `t.Cleanup`).

## Anti-Patterns

### Do not mock what you do not own without a wrapper

Mocking `*sql.DB` directly is fragile. Instead, define your own `Repository` interface and mock that.

### Do not create one giant interface for all database operations

```go
// Bad: too wide
type Database interface {
	GetUser(ctx context.Context, id string) (*User, error)
	CreateUser(ctx context.Context, u *User) error
	GetOrder(ctx context.Context, id string) (*Order, error)
	CreateOrder(ctx context.Context, o *Order) error
	// ... 20 more methods
}

// Good: focused interfaces per consumer
type UserGetter interface {
	GetUser(ctx context.Context, id string) (*User, error)
}
```

### Do not over-verify mock calls

Testing that a mock was called with exact arguments on every method leads to brittle tests that break on refactoring. Verify only the interactions that matter for the behavior you are testing.

### Do not use mocks for simple value types

If a dependency is a pure function or a simple data structure, pass the real thing instead of mocking it.

## Related Patterns

- [core-sdk-go.testing-unit](./core-sdk-go.testing-unit.md) - Unit test structure that uses these mocks
- [core-sdk-go.testing-integration](./core-sdk-go.testing-integration.md) - When to use real services instead of mocks
- [core-sdk-go.testing-fixtures](./core-sdk-go.testing-fixtures.md) - Creating test data for mock return values

## Testing (meta: how to test this pattern)

Regenerate mocks after interface changes:
```bash
go generate ./...
```

Run tests that use mocks:
```bash
go test -v ./...
```

Verify mock expectations are met (gomock and testify do this automatically when tests complete).

---

**Status**: Active
**Compatibility**: Go 1.21+
