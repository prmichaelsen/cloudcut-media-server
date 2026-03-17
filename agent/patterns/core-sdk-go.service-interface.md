# Pattern: Service Interface

**Namespace**: core-sdk-go
**Category**: Service Architecture
**Created**: 2026-03-17
**Status**: Active

---

## Overview

Go interfaces are satisfied implicitly -- there is no `implements` keyword. This pattern leverages implicit satisfaction to define small, focused interfaces at the consumer side. Producers return concrete structs, consumers accept interfaces, and dependency injection happens naturally without frameworks.

---

## Problem

In languages like TypeScript or Java, interfaces are declared at the producer and explicitly implemented. This creates tight coupling between the interface definition and its implementations. It also encourages large, monolithic interfaces that violate the Interface Segregation Principle. Go developers coming from these languages often carry over habits that produce non-idiomatic code.

---

## Solution

Define interfaces where they are used (at the consumer), not where they are implemented (at the producer). Keep interfaces small (1-3 methods). Return concrete structs from constructors. Accept interfaces in function and method parameters. This approach leverages Go's implicit interface satisfaction to decouple packages naturally.

---

## Implementation

### Go Implementation

```go
package order

import "context"

// UserFetcher is defined by the order package -- the consumer.
// It contains only the methods the order package actually needs.
type UserFetcher interface {
	GetByID(ctx context.Context, id string) (*User, error)
}

// User is a minimal representation of a user needed by this package.
type User struct {
	ID    string
	Name  string
	Email string
}

// OrderService depends on UserFetcher, not on the entire user package.
type OrderService struct {
	users UserFetcher
}

func NewOrderService(users UserFetcher) *OrderService {
	return &OrderService{users: users}
}

func (s *OrderService) CreateOrder(ctx context.Context, userID string, items []Item) (*Order, error) {
	// Uses only the GetByID method -- minimal dependency surface
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fetching user %s: %w", userID, err)
	}

	order := &Order{
		UserID:   u.ID,
		UserName: u.Name,
		Items:    items,
	}
	return order, nil
}
```

```go
package user

import "context"

// UserService is a concrete struct. It does NOT declare that it
// implements order.UserFetcher -- Go figures that out automatically.
type UserService struct {
	db *sql.DB
}

func NewUserService(db *sql.DB) *UserService {
	return &UserService{db: db}
}

// GetByID satisfies order.UserFetcher implicitly.
func (s *UserService) GetByID(ctx context.Context, id string) (*User, error) {
	row := s.db.QueryRowContext(ctx, "SELECT id, name, email FROM users WHERE id = $1", id)
	var u User
	if err := row.Scan(&u.ID, &u.Name, &u.Email); err != nil {
		return nil, fmt.Errorf("scanning user %s: %w", id, err)
	}
	return &u, nil
}

// Other methods that order.UserFetcher doesn't need
func (s *UserService) List(ctx context.Context) ([]*User, error) { /* ... */ }
func (s *UserService) Delete(ctx context.Context, id string) error { /* ... */ }
```

### Example Usage

```go
package main

import (
	"database/sql"

	"myapp/order"
	"myapp/user"
)

func main() {
	db, _ := sql.Open("postgres", "...")

	// user.UserService is a concrete struct
	userSvc := user.NewUserService(db)

	// It satisfies order.UserFetcher implicitly -- no cast, no assertion
	orderSvc := order.NewOrderService(userSvc)

	// Use orderSvc...
	_ = orderSvc
}
```

### Repository Interface Pattern

```go
package storage

import "context"

// Repository defines a generic CRUD interface.
// Define this at the consumer, not the storage layer.
type Repository[T any] interface {
	Get(ctx context.Context, id string) (*T, error)
	List(ctx context.Context, opts ListOptions) ([]*T, error)
	Create(ctx context.Context, entity *T) error
	Update(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id string) error
}

type ListOptions struct {
	Limit  int
	Offset int
}
```

```go
package postgres

import (
	"context"
	"database/sql"
)

// ProductStore is a concrete implementation. It does not reference
// storage.Repository -- it just happens to have the right methods.
type ProductStore struct {
	db *sql.DB
}

func NewProductStore(db *sql.DB) *ProductStore {
	return &ProductStore{db: db}
}

func (s *ProductStore) Get(ctx context.Context, id string) (*Product, error) {
	// ... SQL query
	return &Product{}, nil
}

func (s *ProductStore) List(ctx context.Context, opts storage.ListOptions) ([]*Product, error) {
	// ... SQL query with LIMIT/OFFSET
	return nil, nil
}

func (s *ProductStore) Create(ctx context.Context, p *Product) error {
	// ... SQL INSERT
	return nil
}

func (s *ProductStore) Update(ctx context.Context, p *Product) error {
	// ... SQL UPDATE
	return nil
}

func (s *ProductStore) Delete(ctx context.Context, id string) error {
	// ... SQL DELETE
	return nil
}
```

### Compile-Time Interface Satisfaction Check

```go
package postgres

import "myapp/storage"

// Compile-time check that ProductStore satisfies Repository[Product].
var _ storage.Repository[Product] = (*ProductStore)(nil)
```

---

## Benefits

1. Decoupled packages -- consumer defines only what it needs
2. Small interfaces are easier to mock, test, and reason about
3. No import cycles -- consumer owns the interface, producer knows nothing about it
4. Implicit satisfaction means adding a new consumer interface requires zero changes to existing code
5. Encourages the Interface Segregation Principle naturally

---

## Best Practices

**Do define interfaces at the consumer:**
```go
// In the notification package (consumer)
package notification

// Addressable is what notification needs -- just an email.
type Addressable interface {
	Email() string
}

func SendWelcome(to Addressable) error {
	// ...
}
```

**Do keep interfaces small (1-3 methods):**
```go
// GOOD: focused interface
type Reader interface {
	Read(ctx context.Context, id string) (*Entity, error)
}

// GOOD: compose small interfaces when needed
type ReadWriter interface {
	Reader
	Writer
}
```

**Do use compile-time satisfaction checks in producer packages:**
```go
var _ io.ReadCloser = (*MyReader)(nil)
```

**Do return concrete types from constructors:**
```go
// GOOD: returns *UserService, not an interface
func NewUserService(db *sql.DB) *UserService {
	return &UserService{db: db}
}
```

---

## Anti-Patterns

**Don't define interfaces at the producer:**
```go
// BAD: producer-side interface forces all consumers to depend on this package
package user

type UserServiceInterface interface {
	GetByID(ctx context.Context, id string) (*User, error)
	List(ctx context.Context) ([]*User, error)
	Create(ctx context.Context, u *User) error
	Update(ctx context.Context, u *User) error
	Delete(ctx context.Context, id string) error
}
```

**Don't create large "god" interfaces:**
```go
// BAD: 10+ methods means most consumers use only a fraction
type Store interface {
	GetUser(ctx context.Context, id string) (*User, error)
	ListUsers(ctx context.Context) ([]*User, error)
	CreateUser(ctx context.Context, u *User) error
	GetOrder(ctx context.Context, id string) (*Order, error)
	ListOrders(ctx context.Context) ([]*Order, error)
	CreateOrder(ctx context.Context, o *Order) error
	GetProduct(ctx context.Context, id string) (*Product, error)
	// ... and so on
}
```

**Don't return interfaces from constructors:**
```go
// BAD: returning an interface hides the concrete type unnecessarily
func NewUserService(db *sql.DB) UserServiceInterface {
	return &UserService{db: db}
}
```

**Don't use empty interface (any) when you can be specific:**
```go
// BAD: loses all type safety
func Process(data any) error { /* ... */ }

// GOOD: define what you need
type Processable interface {
	Validate() error
	Marshal() ([]byte, error)
}
func Process(data Processable) error { /* ... */ }
```

---

## Related Patterns

- [Service Base](core-sdk-go.service-base.md) -- embedding for lifecycle management
- [Service Container](core-sdk-go.service-container.md) -- wiring interfaces to implementations
- [Service Error Handling](core-sdk-go.service-error-handling.md) -- error interfaces and types

---

## Testing

### Unit Test Example

```go
package order_test

import (
	"context"
	"fmt"
	"testing"

	"myapp/order"
)

// mockUserFetcher is a test double that satisfies order.UserFetcher.
type mockUserFetcher struct {
	users map[string]*order.User
	err   error
}

func (m *mockUserFetcher) GetByID(_ context.Context, id string) (*order.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	u, ok := m.users[id]
	if !ok {
		return nil, fmt.Errorf("user %s not found", id)
	}
	return u, nil
}

func TestOrderService_CreateOrder(t *testing.T) {
	fetcher := &mockUserFetcher{
		users: map[string]*order.User{
			"u1": {ID: "u1", Name: "Alice", Email: "alice@example.com"},
		},
	}

	svc := order.NewOrderService(fetcher)
	ctx := context.Background()

	o, err := svc.CreateOrder(ctx, "u1", []order.Item{{Name: "Widget", Qty: 2}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.UserName != "Alice" {
		t.Errorf("expected UserName Alice, got %s", o.UserName)
	}
}

func TestOrderService_CreateOrder_UserNotFound(t *testing.T) {
	fetcher := &mockUserFetcher{
		users: map[string]*order.User{},
	}

	svc := order.NewOrderService(fetcher)
	ctx := context.Background()

	_, err := svc.CreateOrder(ctx, "unknown", nil)
	if err == nil {
		t.Fatal("expected error for unknown user")
	}
}

func TestOrderService_CreateOrder_FetcherError(t *testing.T) {
	fetcher := &mockUserFetcher{
		err: fmt.Errorf("database connection lost"),
	}

	svc := order.NewOrderService(fetcher)
	ctx := context.Background()

	_, err := svc.CreateOrder(ctx, "u1", nil)
	if err == nil {
		t.Fatal("expected error when fetcher fails")
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
