# Pattern: Client HTTP Transport

**Namespace**: core-sdk-go
**Category**: Client SDK
**Created**: 2026-03-17
**Status**: Active

---

## Overview

Defines the HTTP transport layer for Go SDK clients using the `http.RoundTripper` interface. Custom transports handle cross-cutting concerns like authentication, retry logic, and logging without polluting business logic. Transports compose via decoration (wrapping), forming a middleware chain that processes every outbound HTTP request.

---

## Problem

Every HTTP request to a backend API needs authentication headers, retry logic for transient failures, request/response logging, and error normalization. Scattering this logic across service client methods leads to duplication, inconsistency, and difficulty testing. Go's `net/http` package provides the `RoundTripper` interface but no built-in middleware chain.

---

## Solution

Implement each cross-cutting concern as a standalone `http.RoundTripper` decorator. Each transport wraps an inner `RoundTripper`, adds its behavior, and delegates to the next transport in the chain. Compose transports at client construction time, producing a single `http.Client` that transparently handles auth, retries, and logging for all requests.

---

## Implementation

### Transport Interface

Every transport implements `http.RoundTripper`:

```go
type http.RoundTripper interface {
    RoundTrip(*http.Request) (*http.Response, error)
}
```

### AuthTransport

Adds a Bearer token to every outgoing request.

```go
package transport

import (
	"net/http"
)

// TokenSource provides authentication tokens.
// Implementations can read from config, environment, or a token refresh flow.
type TokenSource interface {
	Token() (string, error)
}

// StaticTokenSource returns a fixed token. Useful for API keys and testing.
type StaticTokenSource struct {
	AccessToken string
}

func (s *StaticTokenSource) Token() (string, error) {
	return s.AccessToken, nil
}

// AuthTransport injects an Authorization header into every request.
type AuthTransport struct {
	Source TokenSource
	Base   http.RoundTripper
}

func (t *AuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.Source.Token()
	if err != nil {
		return nil, fmt.Errorf("transport: obtaining token: %w", err)
	}

	// Clone the request to avoid mutating the caller's request.
	r := req.Clone(req.Context())
	r.Header.Set("Authorization", "Bearer "+token)

	return t.base().RoundTrip(r)
}

func (t *AuthTransport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}
```

### RetryTransport

Retries requests on transient failures with exponential backoff.

```go
package transport

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"time"
)

// RetryTransport retries failed requests with exponential backoff and jitter.
type RetryTransport struct {
	Base       http.RoundTripper
	MaxRetries int           // Maximum number of retry attempts. Default: 3.
	BaseDelay  time.Duration // Initial delay between retries. Default: 500ms.
	MaxDelay   time.Duration // Maximum delay between retries. Default: 30s.
}

func (t *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	maxRetries := t.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}
	baseDelay := t.BaseDelay
	if baseDelay == 0 {
		baseDelay = 500 * time.Millisecond
	}
	maxDelay := t.MaxDelay
	if maxDelay == 0 {
		maxDelay = 30 * time.Second
	}

	var resp *http.Response
	var err error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Clone request for each attempt so the body can be re-read.
		r := req.Clone(req.Context())

		resp, err = t.base().RoundTrip(r)

		if !t.shouldRetry(resp, err) {
			return resp, err
		}

		// Drain and close the response body before retrying.
		if resp != nil {
			resp.Body.Close()
		}

		delay := t.backoff(attempt, baseDelay, maxDelay)
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(delay):
		}
	}

	return resp, err
}

// shouldRetry determines if a request should be retried.
func (t *RetryTransport) shouldRetry(resp *http.Response, err error) bool {
	if err != nil {
		return true // Network errors are retryable.
	}
	// Retry on 429 (rate limit) and 5xx (server errors).
	return resp.StatusCode == http.StatusTooManyRequests ||
		resp.StatusCode >= 500
}

// backoff calculates the delay for the given attempt using exponential backoff with jitter.
func (t *RetryTransport) backoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
	if delay > maxDelay {
		delay = maxDelay
	}
	// Add jitter: +/- 25% of the calculated delay.
	jitter := time.Duration(rand.Int63n(int64(delay) / 2))
	delay = delay/2 + jitter
	return delay
}

func (t *RetryTransport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}
```

### LoggingTransport

Logs outgoing requests and incoming responses using `log/slog`.

```go
package transport

import (
	"log/slog"
	"net/http"
	"time"
)

// LoggingTransport logs request method, URL, status code, and duration.
type LoggingTransport struct {
	Base   http.RoundTripper
	Logger *slog.Logger
}

func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	logger := t.Logger
	if logger == nil {
		logger = slog.Default()
	}

	start := time.Now()
	logger.Info("http request",
		"method", req.Method,
		"url", req.URL.String(),
	)

	resp, err := t.base().RoundTrip(req)
	duration := time.Since(start)

	if err != nil {
		logger.Error("http request failed",
			"method", req.Method,
			"url", req.URL.String(),
			"duration", duration,
			"error", err,
		)
		return nil, err
	}

	logger.Info("http response",
		"method", req.Method,
		"url", req.URL.String(),
		"status", resp.StatusCode,
		"duration", duration,
	)

	return resp, nil
}

func (t *LoggingTransport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}
```

### Composing Transports

Chain transports by nesting them. The outermost transport runs first.

```go
package sdk

import (
	"log/slog"
	"net/http"
	"time"

	"myapp/transport"
)

// NewHTTPClient creates an http.Client with the full transport middleware chain.
// Execution order: logging -> retry -> auth -> http.DefaultTransport
func NewHTTPClient(token string, logger *slog.Logger) *http.Client {
	// Innermost: auth adds the Bearer token.
	auth := &transport.AuthTransport{
		Source: &transport.StaticTokenSource{AccessToken: token},
		Base:   http.DefaultTransport,
	}

	// Middle: retry wraps auth so retries include fresh auth headers.
	retry := &transport.RetryTransport{
		Base:       auth,
		MaxRetries: 3,
		BaseDelay:  500 * time.Millisecond,
		MaxDelay:   30 * time.Second,
	}

	// Outermost: logging wraps everything for full visibility.
	logging := &transport.LoggingTransport{
		Base:   retry,
		Logger: logger,
	}

	return &http.Client{
		Transport: logging,
		Timeout:   30 * time.Second,
	}
}
```

### Error Normalization

Convert HTTP error responses into typed Go errors.

```go
package sdk

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// APIError represents an error response from the API.
type APIError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	RequestID  string `json:"request_id,omitempty"`
}

func (e *APIError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("api error %d (%s): %s [request_id=%s]",
			e.StatusCode, e.Code, e.Message, e.RequestID)
	}
	return fmt.Sprintf("api error %d (%s): %s", e.StatusCode, e.Code, e.Message)
}

// IsNotFound returns true if the error is a 404 response.
func IsNotFound(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}

// IsRateLimit returns true if the error is a 429 response.
func IsRateLimit(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusTooManyRequests
	}
	return false
}

// checkResponse inspects an HTTP response and returns an APIError if the
// status code indicates a failure (4xx or 5xx).
func checkResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	apiErr := &APIError{StatusCode: resp.StatusCode}

	if err := json.Unmarshal(body, apiErr); err != nil {
		// If the body isn't JSON, use the raw body as the message.
		apiErr.Code = http.StatusText(resp.StatusCode)
		apiErr.Message = string(body)
	}

	return apiErr
}
```

---

## Benefits

1. **Separation of concerns** -- each transport handles exactly one responsibility
2. **Composable** -- transports chain in any order via simple struct nesting
3. **Testable** -- each transport can be tested in isolation with a mock RoundTripper
4. **Transparent** -- service client methods never see auth, retry, or logging logic
5. **Standard library compatible** -- everything builds on `net/http.RoundTripper`
6. **Reusable** -- the same transport chain serves all service clients

---

## Best Practices

**Do clone requests before mutating headers:**
```go
func (t *AuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context()) // Clone first
	r.Header.Set("Authorization", "Bearer "+token)
	return t.Base.RoundTrip(r)
}
```

**Do set a timeout on the outermost http.Client:**
```go
client := &http.Client{
	Transport: chain,
	Timeout:   30 * time.Second, // Caps total time including retries
}
```

**Do fall back to http.DefaultTransport when Base is nil:**
```go
func (t *MyTransport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}
```

**Do respect context cancellation in retry loops:**
```go
select {
case <-req.Context().Done():
	return nil, req.Context().Err()
case <-time.After(delay):
}
```

---

## Anti-Patterns

**Don't mutate the original request:**
```go
// BAD: Mutates the caller's request
func (t *AuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+token) // Mutation!
	return t.Base.RoundTrip(req)
}

// GOOD: Clone first
func (t *AuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("Authorization", "Bearer "+token)
	return t.Base.RoundTrip(r)
}
```

**Don't retry non-idempotent requests blindly:**
```go
// BAD: Retries POST requests that may have side effects
func (t *RetryTransport) shouldRetry(resp *http.Response, err error) bool {
	return resp.StatusCode >= 500 // Retries POSTs too!
}

// GOOD: Check method or let the caller opt in
func (t *RetryTransport) shouldRetry(req *http.Request, resp *http.Response, err error) bool {
	if req.Method == http.MethodPost {
		return false // Don't retry non-idempotent methods by default
	}
	return resp.StatusCode >= 500
}
```

**Don't embed business logic in transports:**
```go
// BAD: Transport doing request transformation
func (t *MyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Don't add query params or transform paths here
	req.URL.RawQuery += "&tenant=default"
	return t.Base.RoundTrip(req)
}
```

---

## Related Patterns

- [Service Client](core-sdk-go.client-svc.md) -- consumes the http.Client produced by the transport chain
- [Application Client](core-sdk-go.client-app.md) -- orchestrates multiple service clients, each using transports
- [Service Base](core-sdk-go.service-base.md) -- server-side counterpart to client transports

---

## Testing

### Unit Test: AuthTransport

```go
package transport_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"myapp/transport"
)

// roundTripFunc allows using a function as an http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestAuthTransport_AddsHeader(t *testing.T) {
	var gotHeader string
	inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotHeader = req.Header.Get("Authorization")
		return &http.Response{StatusCode: 200}, nil
	})

	tr := &transport.AuthTransport{
		Source: &transport.StaticTokenSource{AccessToken: "test-token-123"},
		Base:   inner,
	}

	req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/users", nil)
	_, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "Bearer test-token-123"
	if gotHeader != want {
		t.Errorf("Authorization header = %q, want %q", gotHeader, want)
	}
}

func TestAuthTransport_DoesNotMutateOriginalRequest(t *testing.T) {
	inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200}, nil
	})

	tr := &transport.AuthTransport{
		Source: &transport.StaticTokenSource{AccessToken: "secret"},
		Base:   inner,
	}

	req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/users", nil)
	_, _ = tr.RoundTrip(req)

	if req.Header.Get("Authorization") != "" {
		t.Error("original request was mutated")
	}
}
```

### Unit Test: RetryTransport

```go
package transport_test

import (
	"net/http"
	"testing"
	"time"

	"myapp/transport"
)

func TestRetryTransport_RetriesOnServerError(t *testing.T) {
	attempts := 0
	inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts < 3 {
			return &http.Response{
				StatusCode: 503,
				Body:       http.NoBody,
			}, nil
		}
		return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
	})

	tr := &transport.RetryTransport{
		Base:       inner,
		MaxRetries: 3,
		BaseDelay:  1 * time.Millisecond, // Fast for tests
		MaxDelay:   10 * time.Millisecond,
	}

	req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/health", nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}
```

### Integration Test: Full Chain with httptest

```go
package sdk_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"myapp/sdk"
)

func TestHTTPClient_FullChain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := sdk.NewHTTPClient("test-token", logger)

	resp, err := client.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
