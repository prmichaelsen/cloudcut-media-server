# Pattern: Goroutine Lifecycle Management

**Namespace**: core-sdk-go
**Category**: Go-Specific
**Created**: 2026-03-17
**Status**: Active

---

## Overview

Defines patterns for safely starting, managing, and shutting down goroutines. Covers `errgroup.Group` for coordinated goroutines that return errors, `sync.WaitGroup` for fire-and-forget work, worker pools for bounded concurrency, and fan-out/fan-in pipelines. Every goroutine must have a clear owner, a cancellation path, and a defined lifetime.

---

## Problem

Goroutines are cheap to start but easy to leak. Common failure modes include:

- Goroutines blocked on a channel that no one will ever write to
- No mechanism to signal a goroutine to stop when the parent context is cancelled
- Unbounded goroutine creation under load (e.g., one goroutine per incoming request with no limit)
- Lost errors from goroutines that fail silently
- Race conditions when multiple goroutines access shared state without synchronization
- Premature program exit before background goroutines finish their work

---

## Solution

Use structured concurrency patterns where every goroutine has an explicit owner responsible for its lifecycle. Use `errgroup.Group` when goroutines return errors and you need coordinated cancellation. Use `sync.WaitGroup` for fire-and-forget work that does not produce errors. Use worker pools to bound concurrency. Always propagate `context.Context` for cancellation.

---

## Implementation

### errgroup.Group -- Coordinated Goroutines with Error Handling

```go
package fetcher

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"golang.org/x/sync/errgroup"
)

type Result struct {
	URL        string
	StatusCode int
}

// FetchAll fetches multiple URLs concurrently. If any fetch fails, all
// in-flight fetches are cancelled and the first error is returned.
func FetchAll(ctx context.Context, urls []string) ([]Result, error) {
	g, ctx := errgroup.WithContext(ctx)

	var mu sync.Mutex
	results := make([]Result, 0, len(urls))

	for _, url := range urls {
		g.Go(func() error {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return fmt.Errorf("creating request for %s: %w", url, err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("fetching %s: %w", url, err)
			}
			defer resp.Body.Close()

			mu.Lock()
			results = append(results, Result{URL: url, StatusCode: resp.StatusCode})
			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}
```

### sync.WaitGroup -- Fire-and-Forget with Graceful Drain

```go
package notifier

import (
	"context"
	"log/slog"
	"sync"
)

type Notification struct {
	UserID  string
	Message string
}

// Notifier sends notifications asynchronously. Call Close to wait for
// all pending notifications to be sent before shutting down.
type Notifier struct {
	wg     sync.WaitGroup
	logger *slog.Logger
}

func NewNotifier(logger *slog.Logger) *Notifier {
	return &Notifier{logger: logger}
}

// Send enqueues a notification to be sent asynchronously.
func (n *Notifier) Send(ctx context.Context, notif Notification) {
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		if err := sendEmail(ctx, notif); err != nil {
			n.logger.ErrorContext(ctx, "failed to send notification",
				"user_id", notif.UserID,
				"error", err,
			)
		}
	}()
}

// Close blocks until all in-flight notifications have been sent.
func (n *Notifier) Close() {
	n.wg.Wait()
}

func sendEmail(ctx context.Context, notif Notification) error {
	// Implementation omitted -- would call an email service.
	return nil
}
```

### Background Worker with Graceful Shutdown

```go
package worker

import (
	"context"
	"log/slog"
	"time"
)

// QueueProcessor reads from a job queue and processes items until the
// context is cancelled. It is designed to run as a long-lived goroutine.
type QueueProcessor struct {
	pollInterval time.Duration
	logger       *slog.Logger
}

func NewQueueProcessor(pollInterval time.Duration, logger *slog.Logger) *QueueProcessor {
	return &QueueProcessor{
		pollInterval: pollInterval,
		logger:       logger,
	}
}

// Run starts the processing loop. It blocks until ctx is cancelled.
// Returns nil on clean shutdown, or an error if processing fails.
func (q *QueueProcessor) Run(ctx context.Context) error {
	q.logger.InfoContext(ctx, "queue processor started",
		"poll_interval", q.pollInterval,
	)

	ticker := time.NewTicker(q.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			q.logger.InfoContext(ctx, "queue processor shutting down")
			return nil
		case <-ticker.C:
			if err := q.processNext(ctx); err != nil {
				q.logger.ErrorContext(ctx, "processing failed", "error", err)
				// Decide: return err to stop the worker, or continue.
				// Here we continue processing despite individual failures.
			}
		}
	}
}

func (q *QueueProcessor) processNext(ctx context.Context) error {
	// Check context before doing work.
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Simulate fetching and processing a job.
	q.logger.DebugContext(ctx, "polling for next job")
	return nil
}
```

### Fan-Out / Fan-In Pipeline

```go
package pipeline

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/sync/errgroup"
)

type Item struct {
	ID   int
	Data string
}

type ProcessedItem struct {
	ID     int
	Result string
}

// ProcessPipeline demonstrates fan-out/fan-in: items are read from a source,
// processed by multiple workers concurrently, and collected into a single result slice.
func ProcessPipeline(ctx context.Context, items []Item, workerCount int, logger *slog.Logger) ([]ProcessedItem, error) {
	// Fan-out: send items to a channel.
	itemCh := make(chan Item, len(items))
	for _, item := range items {
		itemCh <- item
	}
	close(itemCh)

	// Fan-in: collect results from workers.
	resultCh := make(chan ProcessedItem, len(items))

	g, ctx := errgroup.WithContext(ctx)

	// Start workerCount goroutines to process items.
	for i := 0; i < workerCount; i++ {
		workerID := i
		g.Go(func() error {
			for item := range itemCh {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				logger.DebugContext(ctx, "processing item",
					"worker", workerID,
					"item_id", item.ID,
				)

				result, err := transform(ctx, item)
				if err != nil {
					return fmt.Errorf("worker %d failed on item %d: %w", workerID, item.ID, err)
				}
				resultCh <- result
			}
			return nil
		})
	}

	// Wait for all workers in a separate goroutine so we can close resultCh.
	go func() {
		g.Wait()
		close(resultCh)
	}()

	// Collect results.
	var results []ProcessedItem
	for r := range resultCh {
		results = append(results, r)
	}

	// Return any error from the worker group.
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

func transform(ctx context.Context, item Item) (ProcessedItem, error) {
	select {
	case <-ctx.Done():
		return ProcessedItem{}, ctx.Err()
	default:
	}
	return ProcessedItem{
		ID:     item.ID,
		Result: fmt.Sprintf("processed: %s", item.Data),
	}, nil
}
```

### Bounded Worker Pool with errgroup

```go
package pool

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/sync/errgroup"
)

type Task struct {
	ID      int
	Payload string
}

// RunBounded processes tasks with at most maxWorkers concurrent goroutines.
// errgroup.SetLimit controls the concurrency bound.
func RunBounded(ctx context.Context, tasks []Task, maxWorkers int, logger *slog.Logger) error {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxWorkers)

	for _, task := range tasks {
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			logger.InfoContext(ctx, "executing task", "task_id", task.ID)

			if err := execute(ctx, task); err != nil {
				return fmt.Errorf("task %d failed: %w", task.ID, err)
			}
			return nil
		})
	}

	return g.Wait()
}

func execute(ctx context.Context, task Task) error {
	// Simulate work. In real code, this would call external services, write to DB, etc.
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
```

### Periodic Background Worker with Stop Channel

```go
package background

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// CacheRefresher periodically refreshes a cache. It demonstrates
// starting a goroutine with a clear stop mechanism.
type CacheRefresher struct {
	interval time.Duration
	logger   *slog.Logger
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewCacheRefresher(interval time.Duration, logger *slog.Logger) *CacheRefresher {
	return &CacheRefresher{
		interval: interval,
		logger:   logger,
	}
}

// Start begins the periodic refresh loop in a background goroutine.
func (c *CacheRefresher) Start(ctx context.Context) {
	ctx, c.cancel = context.WithCancel(ctx)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()

		// Refresh immediately on start.
		c.refresh(ctx)

		for {
			select {
			case <-ctx.Done():
				c.logger.InfoContext(ctx, "cache refresher stopped")
				return
			case <-ticker.C:
				c.refresh(ctx)
			}
		}
	}()
}

// Stop signals the background goroutine to stop and waits for it to finish.
func (c *CacheRefresher) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
}

func (c *CacheRefresher) refresh(ctx context.Context) {
	c.logger.DebugContext(ctx, "refreshing cache")
	// Implementation: query database, update in-memory cache, etc.
}
```

---

## Benefits

1. Every goroutine has an explicit owner responsible for its shutdown
2. `errgroup.WithContext` provides automatic cancellation when any goroutine fails
3. `errgroup.SetLimit` prevents unbounded goroutine creation under load
4. Worker pool pattern provides back-pressure without complex channel choreography
5. `sync.WaitGroup` ensures graceful drain of fire-and-forget work before shutdown
6. Fan-out/fan-in is composable and testable with standard library primitives

---

## Best Practices

**Always give goroutines a cancellation path:**
```go
// GOOD: goroutine watches ctx.Done()
go func() {
    for {
        select {
        case <-ctx.Done():
            return
        case item := <-ch:
            process(item)
        }
    }
}()
```

**Use errgroup.SetLimit for bounded concurrency:**
```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(10) // At most 10 concurrent goroutines.
for _, item := range items {
    g.Go(func() error {
        return process(ctx, item)
    })
}
return g.Wait()
```

**Close channels from the sender, not the receiver:**
```go
// GOOD: sender closes when done producing.
go func() {
    defer close(ch)
    for _, item := range items {
        ch <- item
    }
}()

// Receiver simply ranges over the channel.
for item := range ch {
    process(item)
}
```

**Pair every wg.Add with a defer wg.Done:**
```go
wg.Add(1)
go func() {
    defer wg.Done()
    // work...
}()
```

**Return errors from goroutines via errgroup, not via shared variables:**
```go
// GOOD: errors flow through errgroup
g.Go(func() error {
    return mightFail()
})
if err := g.Wait(); err != nil {
    // handle
}
```

---

## Anti-Patterns

**Don't start goroutines without a cancellation mechanism:**
```go
// BAD: This goroutine runs forever. No way to stop it.
go func() {
    for {
        doWork()
        time.Sleep(1 * time.Minute)
    }
}()

// GOOD: Use context for cancellation.
go func() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            doWork()
        }
    }
}()
```

**Don't create unbounded goroutines per request:**
```go
// BAD: Under load, this creates thousands of goroutines.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    go sendAnalytics(r) // No limit, no tracking
    go updateCache(r)   // No limit, no tracking
    // ...
}

// GOOD: Use a bounded worker pool or channel-based dispatcher.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    h.analyticsQueue <- AnalyticsEvent{Request: r} // Back-pressure via channel capacity
}
```

**Don't close a channel from the receiver side:**
```go
// BAD: Receiver closing the channel causes panic if sender writes.
go func() {
    for item := range ch {
        process(item)
    }
    close(ch) // PANIC if sender is still writing
}()

// GOOD: Only the sender closes the channel.
```

**Don't use time.Sleep for goroutine coordination:**
```go
// BAD: Fragile timing-based synchronization.
go doWork()
time.Sleep(2 * time.Second) // "Hopefully it's done by now"

// GOOD: Use sync.WaitGroup or errgroup.
var wg sync.WaitGroup
wg.Add(1)
go func() {
    defer wg.Done()
    doWork()
}()
wg.Wait() // Deterministic synchronization.
```

**Don't ignore the error from errgroup.Wait:**
```go
// BAD: Silently discards goroutine errors.
g, ctx := errgroup.WithContext(ctx)
g.Go(func() error { return riskyOperation(ctx) })
g.Wait() // Error ignored!

// GOOD: Always check the error.
if err := g.Wait(); err != nil {
    return fmt.Errorf("operation failed: %w", err)
}
```

---

## Related Patterns

- [Context Propagation](core-sdk-go.context-propagation.md) -- context.Context as the cancellation mechanism for goroutines
- [Concurrent Services](core-sdk-go.concurrent-services.md) -- errgroup for managing multiple long-lived services
- [Service Base](core-sdk-go.service-base.md) -- BaseService lifecycle that background goroutines participate in

---

## Testing

### Unit Test -- errgroup Cancellation on Error

```go
package fetcher_test

import (
	"context"
	"errors"
	"testing"

	"myapp/fetcher"
)

func TestFetchAll_CancelsOnError(t *testing.T) {
	// Include one invalid URL to trigger an error.
	urls := []string{
		"https://httpbin.org/status/200",
		"://invalid-url", // Will fail.
		"https://httpbin.org/status/200",
	}

	_, err := fetcher.FetchAll(context.Background(), urls)
	if err == nil {
		t.Fatal("expected error from invalid URL")
	}
}
```

### Unit Test -- Bounded Worker Pool Respects Cancellation

```go
package pool_test

import (
	"context"
	"errors"
	"log/slog"
	"io"
	"testing"
	"time"

	"myapp/pool"
)

func TestRunBounded_CancelsOnTimeout(t *testing.T) {
	tasks := make([]pool.Task, 1000)
	for i := range tasks {
		tasks[i] = pool.Task{ID: i, Payload: "data"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := pool.RunBounded(ctx, tasks, 5, logger)
	if err == nil {
		t.Fatal("expected error due to context timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got: %v", err)
	}
}

func TestRunBounded_CompletesAll(t *testing.T) {
	tasks := make([]pool.Task, 10)
	for i := range tasks {
		tasks[i] = pool.Task{ID: i, Payload: "data"}
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := pool.RunBounded(context.Background(), tasks, 3, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

### Unit Test -- Background Worker Shutdown

```go
package background_test

import (
	"context"
	"log/slog"
	"io"
	"testing"
	"time"

	"myapp/background"
)

func TestCacheRefresher_StopsGracefully(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	refresher := background.NewCacheRefresher(50*time.Millisecond, logger)

	ctx := context.Background()
	refresher.Start(ctx)

	// Let it run a few cycles.
	time.Sleep(200 * time.Millisecond)

	// Stop should return promptly without hanging.
	done := make(chan struct{})
	go func() {
		refresher.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success -- stopped cleanly.
	case <-time.After(2 * time.Second):
		t.Fatal("CacheRefresher.Stop did not return in time -- possible goroutine leak")
	}
}
```

### Unit Test -- Fan-Out/Fan-In Pipeline

```go
package pipeline_test

import (
	"context"
	"log/slog"
	"io"
	"testing"

	"myapp/pipeline"
)

func TestProcessPipeline_AllItemsProcessed(t *testing.T) {
	items := []pipeline.Item{
		{ID: 1, Data: "alpha"},
		{ID: 2, Data: "beta"},
		{ID: 3, Data: "gamma"},
		{ID: 4, Data: "delta"},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	results, err := pipeline.ProcessPipeline(context.Background(), items, 2, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != len(items) {
		t.Fatalf("expected %d results, got %d", len(items), len(results))
	}

	// Verify all IDs are present (order may vary due to concurrency).
	seen := make(map[int]bool)
	for _, r := range results {
		seen[r.ID] = true
	}
	for _, item := range items {
		if !seen[item.ID] {
			t.Fatalf("missing result for item %d", item.ID)
		}
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
