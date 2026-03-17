# Pattern: Concurrent Services

**Namespace**: core-sdk-go
**Category**: Go-Specific
**Created**: 2026-03-17
**Status**: Active

---

## Overview

Defines how to run multiple long-lived services (HTTP server, gRPC server, background workers, metrics exporters) concurrently within a single Go process, with coordinated startup, graceful shutdown, and signal handling. Uses `errgroup.Group` as the orchestrator and `context.Context` cancellation as the shutdown signal.

---

## Problem

Production Go applications rarely run a single server. They typically need:

- An HTTP server for REST APIs and health checks
- A gRPC server for internal service-to-service communication
- Background workers for queue processing, cache warming, or scheduled tasks
- A metrics/debug HTTP server on a separate port

Without coordination, these services start and stop independently, leading to:

- Partial availability where some services are up but others are not
- Zombie processes where one service crashes but others keep running
- Unclean shutdown where in-flight requests are dropped
- Signal handling that only stops one of the services
- Health checks that report healthy when critical subsystems have failed

---

## Solution

Use `errgroup.Group` with a shared context to run all services concurrently. When any service returns (due to error or shutdown signal), the shared context is cancelled, triggering graceful shutdown of all other services. OS signal handling (SIGINT, SIGTERM) feeds into the same context cancellation. A health check endpoint aggregates the health of all subsystems.

---

## Implementation

### Complete main.go -- HTTP + gRPC + Worker with Coordinated Shutdown

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	if err := run(logger); err != nil {
		logger.Error("application exited with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	// Create a root context that is cancelled on SIGINT or SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// errgroup.WithContext creates a derived context that is cancelled when
	// any goroutine in the group returns a non-nil error.
	g, ctx := errgroup.WithContext(ctx)

	// ------------------------------------------------------------------
	// Health aggregator -- tracks subsystem health for the /healthz endpoint.
	// ------------------------------------------------------------------
	healthAgg := NewHealthAggregator()

	// ------------------------------------------------------------------
	// 1. HTTP Server (REST API + health check)
	// ------------------------------------------------------------------
	httpServer := newHTTPServer(":8080", healthAgg, logger)
	g.Go(func() error {
		logger.Info("starting HTTP server", "addr", httpServer.Addr)
		healthAgg.SetHealthy("http")
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})
	// Graceful shutdown goroutine for the HTTP server.
	g.Go(func() error {
		<-ctx.Done()
		logger.Info("shutting down HTTP server")
		healthAgg.SetUnhealthy("http", "shutting down")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http server shutdown: %w", err)
		}
		logger.Info("HTTP server stopped")
		return nil
	})

	// ------------------------------------------------------------------
	// 2. gRPC Server
	// ------------------------------------------------------------------
	grpcServer := grpc.NewServer()

	// Register gRPC health service.
	healthSvc := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthSvc)

	// Register your application gRPC services here:
	// pb.RegisterMyServiceServer(grpcServer, myServiceImpl)

	g.Go(func() error {
		lis, err := net.Listen("tcp", ":9090")
		if err != nil {
			return fmt.Errorf("grpc listen: %w", err)
		}
		logger.Info("starting gRPC server", "addr", ":9090")
		healthAgg.SetHealthy("grpc")
		healthSvc.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

		if err := grpcServer.Serve(lis); err != nil {
			return fmt.Errorf("grpc server: %w", err)
		}
		return nil
	})
	// Graceful shutdown goroutine for the gRPC server.
	g.Go(func() error {
		<-ctx.Done()
		logger.Info("shutting down gRPC server")
		healthAgg.SetUnhealthy("grpc", "shutting down")
		healthSvc.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)

		// GracefulStop waits for in-flight RPCs to complete.
		grpcServer.GracefulStop()
		logger.Info("gRPC server stopped")
		return nil
	})

	// ------------------------------------------------------------------
	// 3. Background Worker (queue processor)
	// ------------------------------------------------------------------
	g.Go(func() error {
		logger.Info("starting background worker")
		healthAgg.SetHealthy("worker")
		err := runWorker(ctx, logger)
		healthAgg.SetUnhealthy("worker", "stopped")
		if err != nil {
			return fmt.Errorf("background worker: %w", err)
		}
		return nil
	})

	// ------------------------------------------------------------------
	// Wait for all goroutines to finish.
	// ------------------------------------------------------------------
	logger.Info("all services started, waiting for shutdown signal")
	if err := g.Wait(); err != nil {
		return fmt.Errorf("service group: %w", err)
	}

	logger.Info("all services stopped cleanly")
	return nil
}
```

### HTTP Server with Health Check Endpoint

```go
package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// HealthAggregator tracks the health of multiple subsystems.
type HealthAggregator struct {
	mu       sync.RWMutex
	statuses map[string]SubsystemHealth
}

type SubsystemHealth struct {
	Healthy   bool      `json:"healthy"`
	Message   string    `json:"message,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AggregateHealth struct {
	Healthy    bool                       `json:"healthy"`
	Subsystems map[string]SubsystemHealth `json:"subsystems"`
}

func NewHealthAggregator() *HealthAggregator {
	return &HealthAggregator{
		statuses: make(map[string]SubsystemHealth),
	}
}

func (h *HealthAggregator) SetHealthy(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.statuses[name] = SubsystemHealth{
		Healthy:   true,
		UpdatedAt: time.Now(),
	}
}

func (h *HealthAggregator) SetUnhealthy(name string, message string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.statuses[name] = SubsystemHealth{
		Healthy:   false,
		Message:   message,
		UpdatedAt: time.Now(),
	}
}

// Check returns the aggregate health across all subsystems.
func (h *HealthAggregator) Check() AggregateHealth {
	h.mu.RLock()
	defer h.mu.RUnlock()

	allHealthy := true
	subsystems := make(map[string]SubsystemHealth, len(h.statuses))
	for name, status := range h.statuses {
		subsystems[name] = status
		if !status.Healthy {
			allHealthy = false
		}
	}

	return AggregateHealth{
		Healthy:    allHealthy,
		Subsystems: subsystems,
	}
}

func newHTTPServer(addr string, health *HealthAggregator, logger *slog.Logger) *http.Server {
	mux := http.NewServeMux()

	// Health check endpoint returns 200 when all subsystems are healthy,
	// 503 when any subsystem is unhealthy.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		status := health.Check()
		w.Header().Set("Content-Type", "application/json")

		code := http.StatusOK
		if !status.Healthy {
			code = http.StatusServiceUnavailable
		}
		w.WriteHeader(code)

		if err := json.NewEncoder(w).Encode(status); err != nil {
			logger.ErrorContext(r.Context(), "encoding health response", "error", err)
		}
	})

	// Liveness probe -- always returns 200 if the process is alive.
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Application routes would be registered here:
	// mux.HandleFunc("GET /api/v1/users", usersHandler)

	return &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}
```

### Background Worker

```go
package main

import (
	"context"
	"log/slog"
	"time"
)

// runWorker simulates a background queue processor that polls for work.
// It returns nil when the context is cancelled (graceful shutdown).
func runWorker(ctx context.Context, logger *slog.Logger) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.InfoContext(ctx, "worker received shutdown signal, draining")
			// Perform any final cleanup or drain logic here.
			return nil
		case <-ticker.C:
			if err := processJob(ctx, logger); err != nil {
				logger.ErrorContext(ctx, "job processing failed", "error", err)
				// Continue processing -- individual job failures don't bring down the worker.
			}
		}
	}
}

func processJob(ctx context.Context, logger *slog.Logger) error {
	// Check context before starting expensive work.
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	logger.DebugContext(ctx, "processing job")
	// Simulate job execution.
	return nil
}
```

### Signal Handling Detail

```go
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// setupSignalContext creates a context that is cancelled when the process
// receives SIGINT or SIGTERM. A second signal forces immediate exit.
func setupSignalContext(logger *slog.Logger) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Info("received signal, initiating graceful shutdown", "signal", sig)
		cancel()

		// Second signal: force exit.
		sig = <-sigCh
		logger.Warn("received second signal, forcing exit", "signal", sig)
		os.Exit(1)
	}()

	return ctx, cancel
}
```

### Alternative: Service Runner Abstraction

```go
package runner

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"
)

// Service represents a long-running component that can be started and stopped.
type Service interface {
	// Name returns a human-readable name for logging.
	Name() string
	// Run starts the service and blocks until ctx is cancelled or an error occurs.
	Run(ctx context.Context) error
}

// Runner manages the concurrent execution of multiple services.
type Runner struct {
	services []Service
	logger   *slog.Logger
}

func NewRunner(logger *slog.Logger, services ...Service) *Runner {
	return &Runner{
		services: services,
		logger:   logger,
	}
}

// Run starts all services and waits for a shutdown signal or an error.
// When any service returns, all other services are cancelled.
func (r *Runner) Run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	g, ctx := errgroup.WithContext(ctx)

	for _, svc := range r.services {
		svc := svc
		g.Go(func() error {
			r.logger.Info("starting service", "name", svc.Name())
			if err := svc.Run(ctx); err != nil {
				return fmt.Errorf("service %s: %w", svc.Name(), err)
			}
			r.logger.Info("service stopped", "name", svc.Name())
			return nil
		})
	}

	return g.Wait()
}
```

### Using the Service Runner

```go
package main

import (
	"context"
	"log/slog"
	"os"

	"myapp/runner"
)

// HTTPService wraps an HTTP server to implement the Service interface.
type HTTPService struct {
	addr string
	// ... handler, health checks, etc.
}

func (s *HTTPService) Name() string { return "http" }

func (s *HTTPService) Run(ctx context.Context) error {
	// Start HTTP server, block until ctx is cancelled, then shut down gracefully.
	// (implementation as shown above)
	<-ctx.Done()
	return nil
}

// GRPCService wraps a gRPC server to implement the Service interface.
type GRPCService struct {
	addr string
}

func (s *GRPCService) Name() string { return "grpc" }

func (s *GRPCService) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// WorkerService wraps a background worker to implement the Service interface.
type WorkerService struct{}

func (s *WorkerService) Name() string { return "worker" }

func (s *WorkerService) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func mainWithRunner() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	r := runner.NewRunner(logger,
		&HTTPService{addr: ":8080"},
		&GRPCService{addr: ":9090"},
		&WorkerService{},
	)

	if err := r.Run(context.Background()); err != nil {
		logger.Error("application error", "error", err)
		os.Exit(1)
	}
}
```

---

## Benefits

1. Single `errgroup` ensures all services start and stop together -- no orphaned processes
2. If any service crashes, all others are automatically signalled to shut down via context cancellation
3. OS signals (SIGINT, SIGTERM) integrate into the same cancellation flow
4. `http.Server.Shutdown` and `grpc.Server.GracefulStop` allow in-flight requests to complete
5. Aggregated health check gives load balancers and orchestrators a single endpoint to probe
6. Separate liveness (`/livez`) and readiness (`/healthz`) endpoints follow Kubernetes conventions
7. The `Service` interface abstraction makes it trivial to add new services without modifying `main`

---

## Best Practices

**Use signal.NotifyContext for clean signal handling:**
```go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()
// ctx is cancelled when SIGINT or SIGTERM is received.
```

**Give shutdown a timeout to prevent hanging:**
```go
// Don't wait forever for in-flight requests.
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
httpServer.Shutdown(shutdownCtx)
```

**Use a fresh context.Background() for shutdown, not the cancelled parent:**
```go
// BAD: The parent context is already cancelled -- Shutdown would return immediately.
<-ctx.Done()
httpServer.Shutdown(ctx) // Already cancelled!

// GOOD: Create a new context with a timeout for the shutdown window.
<-ctx.Done()
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
httpServer.Shutdown(shutdownCtx)
```

**Set server timeouts to prevent resource exhaustion:**
```go
server := &http.Server{
    ReadTimeout:  10 * time.Second,
    WriteTimeout: 30 * time.Second,
    IdleTimeout:  60 * time.Second,
}
```

**Log service lifecycle transitions for observability:**
```go
logger.Info("starting service", "name", svc.Name())
// ... run ...
logger.Info("service stopped", "name", svc.Name())
```

---

## Anti-Patterns

**Don't start services without coordinated shutdown:**
```go
// BAD: Services run independently. If one crashes, others keep running.
go httpServer.ListenAndServe()
go grpcServer.Serve(lis)
go runWorker(ctx)
select {} // Block forever -- no coordination.
```

**Don't use os.Exit for shutdown:**
```go
// BAD: os.Exit skips deferred functions and does not drain connections.
signal.Notify(sigCh, syscall.SIGINT)
go func() {
    <-sigCh
    os.Exit(0) // Connections dropped, defers skipped!
}()
```

**Don't ignore errors from services:**
```go
// BAD: Error is silently swallowed.
g.Go(func() error {
    httpServer.ListenAndServe() // Error not returned!
    return nil
})

// GOOD: Propagate the error.
g.Go(func() error {
    if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        return fmt.Errorf("http server: %w", err)
    }
    return nil
})
```

**Don't hardcode ports -- use configuration:**
```go
// BAD
httpServer := &http.Server{Addr: ":8080"}

// GOOD
httpServer := &http.Server{Addr: cfg.HTTPAddr}
```

**Don't forget to handle http.ErrServerClosed:**
```go
// BAD: Treats graceful shutdown as an error.
if err := httpServer.ListenAndServe(); err != nil {
    return err // Returns error even on clean shutdown!
}

// GOOD: Filter out the expected shutdown error.
if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
    return err
}
```

---

## Related Patterns

- [Service Base](core-sdk-go.service-base.md) -- BaseService lifecycle that individual services can embed
- [Goroutine Lifecycle](core-sdk-go.goroutine-lifecycle.md) -- goroutine management patterns used within each service
- [Context Propagation](core-sdk-go.context-propagation.md) -- context flows from signal handling through the entire service tree

---

## Testing

### Integration Test -- Coordinated Startup and Shutdown

```go
package main_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"log/slog"

	"golang.org/x/sync/errgroup"
)

func TestHTTPServer_HealthEndpoint(t *testing.T) {
	healthAgg := NewHealthAggregator()
	healthAgg.SetHealthy("http")
	healthAgg.SetHealthy("grpc")
	healthAgg.SetHealthy("worker")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := newHTTPServer(":0", healthAgg, logger) // Port 0 = random available port

	// Use a listener to get the actual port.
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go server.Serve(ln)
	defer server.Close()

	addr := ln.Addr().String()

	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var health AggregateHealth
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("decoding health response: %v", err)
	}

	if !health.Healthy {
		t.Fatal("expected healthy aggregate status")
	}
	if len(health.Subsystems) != 3 {
		t.Fatalf("expected 3 subsystems, got %d", len(health.Subsystems))
	}
}

func TestHTTPServer_UnhealthySubsystem(t *testing.T) {
	healthAgg := NewHealthAggregator()
	healthAgg.SetHealthy("http")
	healthAgg.SetUnhealthy("grpc", "connection refused")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := newHTTPServer(":0", healthAgg, logger)

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go server.Serve(ln)
	defer server.Close()

	addr := ln.Addr().String()

	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
}
```

### Unit Test -- Service Runner

```go
package runner_test

import (
	"context"
	"errors"
	"log/slog"
	"io"
	"testing"
	"time"

	"myapp/runner"
)

type fakeService struct {
	name string
	err  error
}

func (f *fakeService) Name() string { return f.name }

func (f *fakeService) Run(ctx context.Context) error {
	if f.err != nil {
		return f.err
	}
	<-ctx.Done()
	return nil
}

func TestRunner_AllServicesStopOnCancel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := runner.NewRunner(logger,
		&fakeService{name: "svc-a"},
		&fakeService{name: "svc-b"},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := r.Run(ctx)
	// All services should stop cleanly when context is cancelled.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunner_ErrorInOneStopsAll(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := runner.NewRunner(logger,
		&fakeService{name: "healthy"},
		&fakeService{name: "failing", err: errors.New("crash")},
	)

	err := r.Run(context.Background())
	if err == nil {
		t.Fatal("expected error from failing service")
	}
	if !errors.Is(err, errors.Unwrap(err)) {
		// Just verify we got an error -- the exact wrapping depends on errgroup.
	}
}
```

### Unit Test -- Graceful Shutdown Timing

```go
package main_test

import (
	"context"
	"log/slog"
	"io"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
)

func TestGracefulShutdown_CompletesWithinTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	healthAgg := NewHealthAggregator()
	httpServer := newHTTPServer(":0", healthAgg, logger)

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return httpServer.Serve(ln)
	})
	g.Go(func() error {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		return httpServer.Shutdown(shutdownCtx)
	})

	// Cancel the context to trigger shutdown.
	cancel()

	done := make(chan error, 1)
	go func() {
		done <- g.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("shutdown error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown did not complete within 10 seconds")
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
