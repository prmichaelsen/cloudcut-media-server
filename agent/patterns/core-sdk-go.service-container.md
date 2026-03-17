# Pattern: Service Container

**Namespace**: core-sdk-go
**Category**: Dependency Injection
**Created**: 2026-03-17
**Status**: Active

---

## Overview

Dependency injection in Go without magic. This pattern covers three approaches: manual constructor injection (preferred for small-to-medium projects), compile-time code generation with google/wire (for larger projects), and a Container struct that holds all services and manages their lifecycle. Reflection-heavy DI frameworks are explicitly discouraged.

---

## Problem

As applications grow, constructing the dependency graph becomes tedious and error-prone. Services depend on other services, repositories, configuration, and loggers. Without a structured approach, `main()` becomes a tangled mess of constructor calls, and refactoring dependencies requires changes scattered across the codebase.

---

## Solution

Use a `Container` struct as the single place where all services are constructed and wired together. For small projects, manual constructor injection inside `NewContainer()` is sufficient. For larger projects, use google/wire to generate the wiring code at compile time. Avoid reflection-based DI containers (e.g., uber-go/dig) as they move errors from compile time to runtime.

---

## Implementation

### Go Implementation

#### Approach 1: Manual Constructor Injection (Recommended for Most Projects)

```go
package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"myapp/order"
	"myapp/product"
	"myapp/user"
)

// Config holds all application configuration.
type Config struct {
	DatabaseURL string
	LogLevel    slog.Level
	Port        int
}

// Container holds all application services and manages their lifecycle.
type Container struct {
	Config     Config
	Logger     *slog.Logger
	DB         *sql.DB
	UserSvc    *user.UserService
	ProductSvc *product.ProductService
	OrderSvc   *order.OrderService
}

// NewContainer constructs all services and wires dependencies.
// This is the single place where the entire dependency graph is assembled.
func NewContainer(cfg Config) (*Container, error) {
	// Logger
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	})
	logger := slog.New(handler)

	// Database
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Repositories (concrete structs)
	userRepo := user.NewPostgresRepository(db)
	productRepo := product.NewPostgresRepository(db)
	orderRepo := order.NewPostgresRepository(db)

	// Services (wired with their dependencies)
	userSvc := user.NewUserService(userRepo, logger.With("service", "user"))
	productSvc := product.NewProductService(productRepo, logger.With("service", "product"))
	orderSvc := order.NewOrderService(orderRepo, userSvc, productSvc, logger.With("service", "order"))

	return &Container{
		Config:     cfg,
		Logger:     logger,
		DB:         db,
		UserSvc:    userSvc,
		ProductSvc: productSvc,
		OrderSvc:   orderSvc,
	}, nil
}

// Init initializes all services in dependency order.
func (c *Container) Init(ctx context.Context) error {
	services := []interface {
		Init(context.Context) error
	}{
		c.UserSvc,
		c.ProductSvc,
		c.OrderSvc,
	}

	for _, svc := range services {
		if err := svc.Init(ctx); err != nil {
			return fmt.Errorf("initializing service: %w", err)
		}
	}
	return nil
}

// Close shuts down all services in reverse order.
func (c *Container) Close() error {
	var errs []error

	closers := []interface {
		Close() error
	}{
		c.OrderSvc,
		c.ProductSvc,
		c.UserSvc,
	}

	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if err := c.DB.Close(); err != nil {
		errs = append(errs, fmt.Errorf("closing database: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}
```

#### Approach 2: google/wire for Compile-Time DI (Larger Projects)

```go
// wire.go (input to wire code generator -- not compiled directly)
//go:build wireinject

package app

import (
	"github.com/google/wire"
	"myapp/order"
	"myapp/product"
	"myapp/user"
)

// ProviderSet groups related providers.
var UserSet = wire.NewSet(
	user.NewPostgresRepository,
	wire.Bind(new(user.Repository), new(*user.PostgresRepository)),
	user.NewUserService,
)

var ProductSet = wire.NewSet(
	product.NewPostgresRepository,
	wire.Bind(new(product.Repository), new(*product.PostgresRepository)),
	product.NewProductService,
)

var OrderSet = wire.NewSet(
	order.NewPostgresRepository,
	wire.Bind(new(order.Repository), new(*order.PostgresRepository)),
	order.NewOrderService,
)

var AppSet = wire.NewSet(
	UserSet,
	ProductSet,
	OrderSet,
	NewContainer,
)

// InitializeContainer is the injector function. Wire generates the body.
func InitializeContainer(cfg Config) (*Container, error) {
	wire.Build(AppSet)
	return nil, nil // wire replaces this
}
```

```bash
# Generate the wiring code
$ wire ./...
# This produces wire_gen.go with the actual constructor calls
```

### Example Usage

```go
package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"myapp/app"
)

func main() {
	cfg := app.Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		LogLevel:    slog.LevelInfo,
		Port:        8080,
	}

	container, err := app.NewContainer(cfg)
	if err != nil {
		log.Fatalf("creating container: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := container.Init(ctx); err != nil {
		log.Fatalf("initializing services: %v", err)
	}

	// Graceful shutdown on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		container.Logger.Info("received shutdown signal")
		cancel()
		if err := container.Close(); err != nil {
			container.Logger.Error("shutdown error", "error", err)
		}
	}()

	// Start HTTP server, gRPC server, etc. using container.OrderSvc, etc.
	log.Printf("server listening on :%d", cfg.Port)
	select {}
}
```

---

## Benefits

1. Single place to see the entire dependency graph (NewContainer)
2. Compile-time safety -- missing dependencies are caught before runtime
3. No reflection magic -- easy to debug and understand
4. Lifecycle management (Init/Close) in deterministic order
5. google/wire option scales to large codebases without runtime cost
6. Clear separation between construction (container) and behavior (services)

---

## Best Practices

**Do keep NewContainer as the single wiring point:**
```go
// GOOD: all construction in one place
func NewContainer(cfg Config) (*Container, error) {
	repo := user.NewPostgresRepository(db)
	svc := user.NewUserService(repo, logger)
	return &Container{UserSvc: svc}, nil
}
```

**Do use constructor injection (pass dependencies as parameters):**
```go
// GOOD: dependencies are explicit
func NewUserService(repo UserRepository, logger *slog.Logger) *UserService {
	return &UserService{repo: repo, logger: logger}
}
```

**Do shut down in reverse initialization order:**
```go
// Init order:  DB -> UserSvc -> OrderSvc
// Close order: OrderSvc -> UserSvc -> DB
```

**Do use wire.Bind to connect interfaces to implementations:**
```go
var Set = wire.NewSet(
	NewPostgresRepo,
	wire.Bind(new(Repository), new(*PostgresRepo)),
)
```

---

## Anti-Patterns

**Don't use reflection-based DI containers:**
```go
// BAD: uber-go/dig uses reflection -- errors at runtime, not compile time
container := dig.New()
container.Provide(NewUserService)
container.Provide(NewOrderService)
// Errors only discovered when Invoke is called
container.Invoke(func(svc *OrderService) {
	// ...
})
```

**Don't use global singletons:**
```go
// BAD: global state makes testing difficult and hides dependencies
var globalDB *sql.DB
var globalUserSvc *UserService

func init() {
	globalDB, _ = sql.Open("postgres", os.Getenv("DB"))
	globalUserSvc = NewUserService(globalDB)
}
```

**Don't construct dependencies inside service methods:**
```go
// BAD: service creates its own dependency -- can't test, can't swap
func (s *OrderService) CreateOrder(ctx context.Context) error {
	userSvc := user.NewUserService(sql.Open("postgres", "..."))
	u, err := userSvc.GetByID(ctx, "123")
	// ...
}
```

**Don't pass the container itself as a dependency:**
```go
// BAD: service locator pattern -- hides actual dependencies
func NewOrderService(c *Container) *OrderService {
	return &OrderService{container: c}
}

func (s *OrderService) Handle(ctx context.Context) {
	// Reaches into container for whatever it needs -- opaque
	user, _ := s.container.UserSvc.GetByID(ctx, "123")
}
```

---

## Related Patterns

- [Service Base](core-sdk-go.service-base.md) -- lifecycle management for individual services
- [Service Interface](core-sdk-go.service-interface.md) -- defining the interfaces that get wired
- [Service Error Handling](core-sdk-go.service-error-handling.md) -- handling errors during init/close
- [Service Logging](core-sdk-go.service-logging.md) -- passing loggers through the container

---

## Testing

### Unit Test Example

```go
package app_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"myapp/app"
)

// testContainer creates a Container with test doubles.
func testContainer(t *testing.T) *app.Container {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Use in-memory or stub implementations
	userRepo := user.NewInMemoryRepository()
	productRepo := product.NewInMemoryRepository()
	orderRepo := order.NewInMemoryRepository()

	userSvc := user.NewUserService(userRepo, logger)
	productSvc := product.NewProductService(productRepo, logger)
	orderSvc := order.NewOrderService(orderRepo, userSvc, productSvc, logger)

	return &app.Container{
		Logger:     logger,
		UserSvc:    userSvc,
		ProductSvc: productSvc,
		OrderSvc:   orderSvc,
	}
}

func TestContainer_InitAndClose(t *testing.T) {
	c := testContainer(t)
	ctx := context.Background()

	if err := c.Init(ctx); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Verify all services are running
	if c.UserSvc.State() != service.StateRunning {
		t.Error("user service not running")
	}
	if c.OrderSvc.State() != service.StateRunning {
		t.Error("order service not running")
	}

	if err := c.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	if c.UserSvc.State() != service.StateStopped {
		t.Error("user service not stopped")
	}
}

func TestContainer_InitFailure_PartialCleanup(t *testing.T) {
	c := testContainer(t)
	ctx := context.Background()

	// Simulate a failing service by using a broken repo
	c.OrderSvc = order.NewOrderService(
		&brokenRepo{err: fmt.Errorf("connection refused")},
		c.UserSvc,
		c.ProductSvc,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	err := c.Init(ctx)
	if err == nil {
		t.Fatal("expected init error")
	}

	// Verify cleanup of already-initialized services
	if closeErr := c.Close(); closeErr != nil {
		t.Logf("close returned: %v (expected for partial init)", closeErr)
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
