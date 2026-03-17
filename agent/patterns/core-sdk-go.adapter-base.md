# Pattern: Base Adapter

**Namespace**: core-sdk-go
**Category**: Adapter Layer
**Created**: 2026-03-17
**Status**: Active

---

## Overview

The Base Adapter pattern defines a common interface and shared struct for all adapters in a Go application. An adapter is anything that exposes the application's service layer to the outside world: an HTTP server, a gRPC server, a CLI tool, a message consumer, etc. By standardizing lifecycle management (start, stop, health) behind a single interface, the application can orchestrate multiple adapters uniformly.

## Problem

Go applications often need to expose the same business logic through multiple transports. Without a common abstraction, each transport has its own ad-hoc startup/shutdown logic, health reporting, and signal handling. This leads to duplicated boilerplate, inconsistent graceful shutdown behavior, and difficulty composing multiple servers in a single process.

## Solution

Define an `Adapter` interface with three methods: `Start`, `Stop`, and `Health`. Provide a `BaseAdapter` struct that embeds common fields (logger, config, health state) so concrete adapters only implement transport-specific logic. Use `context.Context` for cancellation and `os.Signal` for system signal handling.

## Implementation

### The Adapter Interface

```go
package adapter

import (
	"context"
	"time"
)

// HealthStatus represents the health of an adapter.
type HealthStatus struct {
	Healthy   bool      `json:"healthy"`
	Message   string    `json:"message,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

// Adapter defines the lifecycle contract for any transport adapter.
type Adapter interface {
	// Start begins serving. It blocks until the context is cancelled
	// or a fatal error occurs.
	Start(ctx context.Context) error

	// Stop performs graceful shutdown within the given context deadline.
	Stop(ctx context.Context) error

	// Health returns the current health status.
	Health() HealthStatus
}
```

### The BaseAdapter Struct

```go
package adapter

import (
	"log/slog"
	"sync"
	"time"
)

// BaseAdapter provides common functionality for all adapters.
type BaseAdapter struct {
	Name   string
	Logger *slog.Logger

	mu      sync.RWMutex
	healthy bool
	message string
}

// NewBaseAdapter creates a BaseAdapter with sensible defaults.
func NewBaseAdapter(name string, logger *slog.Logger) BaseAdapter {
	if logger == nil {
		logger = slog.Default()
	}
	return BaseAdapter{
		Name:    name,
		Logger:  logger,
		healthy: true,
		message: "initialized",
	}
}

// Health returns the current health status. Safe for concurrent use.
func (b *BaseAdapter) Health() HealthStatus {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return HealthStatus{
		Healthy:   b.healthy,
		Message:   b.message,
		CheckedAt: time.Now(),
	}
}

// SetHealth updates the health state. Safe for concurrent use.
func (b *BaseAdapter) SetHealth(healthy bool, message string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.healthy = healthy
	b.message = message
}
```

### Signal Handling and Multi-Adapter Orchestration

```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"myapp/adapter"
)

// App orchestrates multiple adapters.
type App struct {
	adapters []adapter.Adapter
	logger   *slog.Logger
}

func (a *App) Run() error {
	// Create a context that cancels on SIGINT or SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start all adapters concurrently.
	errCh := make(chan error, len(a.adapters))
	for _, ad := range a.adapters {
		go func(ad adapter.Adapter) {
			if err := ad.Start(ctx); err != nil {
				errCh <- fmt.Errorf("adapter failed: %w", err)
			}
		}(ad)
	}

	// Wait for context cancellation (signal received) or a fatal error.
	select {
	case <-ctx.Done():
		a.logger.Info("shutdown signal received")
	case err := <-errCh:
		a.logger.Error("adapter error, shutting down", "error", err)
	}

	// Graceful shutdown with a deadline.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for _, ad := range a.adapters {
		wg.Add(1)
		go func(ad adapter.Adapter) {
			defer wg.Done()
			if err := ad.Stop(shutdownCtx); err != nil {
				a.logger.Error("adapter stop error", "error", err)
			}
		}(ad)
	}
	wg.Wait()

	return nil
}
```

### Concrete Adapter Example (HTTP Server)

```go
package httpadapter

import (
	"context"
	"net"
	"net/http"
	"time"

	"log/slog"
	"myapp/adapter"
)

// HTTPAdapter serves HTTP traffic.
type HTTPAdapter struct {
	adapter.BaseAdapter
	server *http.Server
	addr   string
}

func New(addr string, handler http.Handler, logger *slog.Logger) *HTTPAdapter {
	return &HTTPAdapter{
		BaseAdapter: adapter.NewBaseAdapter("http", logger),
		server: &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		addr: addr,
	}
}

func (h *HTTPAdapter) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", h.addr)
	if err != nil {
		h.SetHealth(false, err.Error())
		return err
	}

	h.SetHealth(true, "listening on "+h.addr)
	h.Logger.Info("http adapter started", "addr", h.addr)

	// Serve until context is cancelled.
	errCh := make(chan error, 1)
	go func() { errCh <- h.server.Serve(ln) }()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			h.SetHealth(false, err.Error())
			return err
		}
		return nil
	}
}

func (h *HTTPAdapter) Stop(ctx context.Context) error {
	h.Logger.Info("http adapter stopping")
	h.SetHealth(false, "shutting down")
	return h.server.Shutdown(ctx)
}
```

### Concrete Adapter Example (gRPC Server)

```go
package grpcadapter

import (
	"context"
	"net"

	"log/slog"
	"myapp/adapter"

	"google.golang.org/grpc"
)

type GRPCAdapter struct {
	adapter.BaseAdapter
	server *grpc.Server
	addr   string
}

func New(addr string, server *grpc.Server, logger *slog.Logger) *GRPCAdapter {
	return &GRPCAdapter{
		BaseAdapter: adapter.NewBaseAdapter("grpc", logger),
		server:      server,
		addr:        addr,
	}
}

func (g *GRPCAdapter) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", g.addr)
	if err != nil {
		g.SetHealth(false, err.Error())
		return err
	}

	g.SetHealth(true, "listening on "+g.addr)
	g.Logger.Info("grpc adapter started", "addr", g.addr)

	errCh := make(chan error, 1)
	go func() { errCh <- g.server.Serve(ln) }()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		g.SetHealth(false, err.Error())
		return err
	}
}

func (g *GRPCAdapter) Stop(ctx context.Context) error {
	g.Logger.Info("grpc adapter stopping")
	g.SetHealth(false, "shutting down")
	g.server.GracefulStop()
	return nil
}
```

### Concrete Adapter Example (CLI)

```go
package cliadapter

import (
	"context"
	"log/slog"

	"myapp/adapter"
)

type CLIAdapter struct {
	adapter.BaseAdapter
	runFn func(ctx context.Context) error
}

func New(runFn func(ctx context.Context) error, logger *slog.Logger) *CLIAdapter {
	return &CLIAdapter{
		BaseAdapter: adapter.NewBaseAdapter("cli", logger),
		runFn:       runFn,
	}
}

func (c *CLIAdapter) Start(ctx context.Context) error {
	c.SetHealth(true, "running")
	err := c.runFn(ctx)
	c.SetHealth(false, "completed")
	return err
}

func (c *CLIAdapter) Stop(_ context.Context) error {
	// CLI runs to completion; nothing to shut down.
	return nil
}
```

## Benefits

1. **Uniform Lifecycle**: All transports start, stop, and report health the same way, simplifying orchestration.
2. **Graceful Shutdown**: Context-based cancellation ensures all adapters shut down cleanly on signals.
3. **Composability**: Run an HTTP server, gRPC server, and background worker in one binary without ad-hoc wiring.
4. **Testability**: The `Adapter` interface is easy to mock in integration tests.
5. **Health Aggregation**: A health endpoint can iterate over all adapters and report composite status.

## Best Practices

- Always propagate `context.Context` from `Start` into underlying servers and handlers.
- Set read/write timeouts on HTTP servers; do not rely on the zero-value defaults.
- Use `signal.NotifyContext` (Go 1.16+) instead of manually creating signal channels.
- Log at adapter boundaries: log when starting, when healthy, and when stopping.
- Keep `Stop` idempotent -- calling it twice should not panic or return an error.

## Anti-Patterns

### Ignoring the Stop Context Deadline

**Bad**: Calling `Stop` with `context.Background()` and no timeout, risking an indefinite hang.

```go
// Bad: no deadline on shutdown
ad.Stop(context.Background())
```

**Good**: Always use a timeout context for shutdown.

```go
ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()
ad.Stop(ctx)
```

### Embedding Business Logic in Adapters

**Bad**: Putting service-layer logic directly in the adapter.

**Good**: Adapters only translate between the transport protocol and the service interface. Inject services as dependencies.

### Swallowing Errors from Start

**Bad**: Starting an adapter in a goroutine and never checking if it returned an error.

**Good**: Always collect errors from `Start` via a channel or errgroup.

## Related Patterns

- **[adapter-rest](./core-sdk-go.adapter-rest.md)**: HTTP REST adapter built on this base.
- **[adapter-mcp](./core-sdk-go.adapter-mcp.md)**: MCP server adapter built on this base.
- **[adapter-cli](./core-sdk-go.adapter-cli.md)**: CLI adapter built on this base.
- **[adapter-client](./core-sdk-go.adapter-client.md)**: Client SDK adapter built on this base.

## Testing

### Unit Testing the BaseAdapter

```go
package adapter_test

import (
	"testing"

	"myapp/adapter"
)

func TestBaseAdapter_Health(t *testing.T) {
	ba := adapter.NewBaseAdapter("test", nil)

	h := ba.Health()
	if !h.Healthy {
		t.Fatal("expected healthy after init")
	}

	ba.SetHealth(false, "broken")
	h = ba.Health()
	if h.Healthy {
		t.Fatal("expected unhealthy after SetHealth(false)")
	}
	if h.Message != "broken" {
		t.Fatalf("expected message 'broken', got %q", h.Message)
	}
}
```

### Integration Testing with a Mock Adapter

```go
package adapter_test

import (
	"context"
	"testing"
	"time"

	"myapp/adapter"
)

type mockAdapter struct {
	adapter.BaseAdapter
	started chan struct{}
}

func (m *mockAdapter) Start(ctx context.Context) error {
	m.SetHealth(true, "running")
	close(m.started)
	<-ctx.Done()
	return nil
}

func (m *mockAdapter) Stop(ctx context.Context) error {
	m.SetHealth(false, "stopped")
	return nil
}

func TestAdapter_Lifecycle(t *testing.T) {
	m := &mockAdapter{
		BaseAdapter: adapter.NewBaseAdapter("mock", nil),
		started:     make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	go m.Start(ctx)

	select {
	case <-m.started:
	case <-time.After(time.Second):
		t.Fatal("adapter did not start in time")
	}

	if !m.Health().Healthy {
		t.Fatal("expected healthy")
	}

	cancel()
	time.Sleep(50 * time.Millisecond)

	shutCtx, shutCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutCancel()
	if err := m.Stop(shutCtx); err != nil {
		t.Fatalf("stop error: %v", err)
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
