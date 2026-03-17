# Pattern: Client Adapter

**Namespace**: core-sdk-go
**Category**: Adapter Layer
**Created**: 2026-03-17
**Status**: Active

---

## Overview

The Client Adapter pattern wraps a remote API (typically REST) as an idiomatic Go client library. It uses exported structs with methods, the functional options pattern for configuration, and resource sub-clients for logical grouping. This pattern is the "other side" of the REST adapter: while the REST adapter translates inbound HTTP to service calls, the Client adapter translates outbound method calls to HTTP requests.

## Problem

Consumers of a Go API need a type-safe, ergonomic client that handles HTTP boilerplate: constructing URLs, serializing request bodies, parsing responses, setting headers, managing timeouts, and retrying transient failures. Without a dedicated client library, every consumer reimplements this logic, leading to inconsistent error handling, missed headers, and duplicated code.

## Solution

Create a `Client` struct that owns an `*http.Client`, base URL, and configuration. Use the functional options pattern (`WithTimeout`, `WithLogger`, etc.) for flexible construction. Group operations by resource (`client.Users.Get()`, `client.Projects.List()`) using sub-client structs. Each method builds a request, executes it, and maps the response (or error) to a Go type.

## Implementation

### Project Structure

```
pkg/
  client/
    client.go          # Client struct, constructor, options
    users.go           # UsersService sub-client
    projects.go        # ProjectsService sub-client
    errors.go          # Error types
    request.go         # Request/response helpers
```

### Functional Options

```go
package client

import (
	"log/slog"
	"net/http"
	"time"
)

// Option configures the Client.
type Option func(*Client)

// WithHTTPClient sets a custom http.Client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// WithLogger sets a structured logger.
func WithLogger(logger *slog.Logger) Option {
	return func(c *Client) {
		c.logger = logger
	}
}

// WithAuthToken sets a Bearer token for all requests.
func WithAuthToken(token string) Option {
	return func(c *Client) {
		c.authToken = token
	}
}

// WithUserAgent sets a custom User-Agent header.
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		c.userAgent = ua
	}
}

// WithBaseURL overrides the default base URL (useful for testing).
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}
```

### The Client Struct

```go
package client

import (
	"log/slog"
	"net/http"
	"time"
)

const (
	defaultBaseURL   = "https://api.myapp.com"
	defaultTimeout   = 30 * time.Second
	defaultUserAgent = "myapp-go-client/1.0"
	apiVersion       = "v1"
)

// Client is the MyApp API client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	authToken  string
	userAgent  string
	logger     *slog.Logger

	// Resource sub-clients
	Users    *UsersService
	Projects *ProjectsService
}

// New creates a new Client with the given options.
func New(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    defaultBaseURL,
		userAgent:  defaultUserAgent,
		logger:     slog.Default(),
	}

	for _, opt := range opts {
		opt(c)
	}

	// Initialize sub-clients, sharing the parent's transport config.
	c.Users = &UsersService{client: c}
	c.Projects = &ProjectsService{client: c}

	return c
}
```

### Request and Response Helpers

```go
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// apiURL constructs a full API URL: baseURL/api/v1/path.
func (c *Client) apiURL(path string) string {
	return fmt.Sprintf("%s/api/%s/%s", c.baseURL, apiVersion, path)
}

// newRequest creates an authenticated HTTP request.
func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	u := c.apiURL(path)

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	return req, nil
}

// do executes the request and decodes the response into v.
func (c *Client) do(req *http.Request, v any) error {
	c.logger.Debug("request",
		"method", req.Method,
		"url", req.URL.String(),
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return parseAPIError(resp)
	}

	if v != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// addQueryParams appends query parameters to a URL path.
func addQueryParams(path string, params map[string]string) string {
	if len(params) == 0 {
		return path
	}
	v := url.Values{}
	for key, val := range params {
		if val != "" {
			v.Set(key, val)
		}
	}
	return path + "?" + v.Encode()
}
```

### Error Types

```go
package client

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// APIError represents an error response from the API.
type APIError struct {
	StatusCode int    `json:"code"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("api error (status %d): %s", e.StatusCode, e.Message)
}

// IsNotFound returns true if the error is a 404.
func IsNotFound(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}

// IsConflict returns true if the error is a 409.
func IsConflict(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == http.StatusConflict
	}
	return false
}

// IsUnauthorized returns true if the error is a 401.
func IsUnauthorized(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == http.StatusUnauthorized
	}
	return false
}

// parseAPIError reads the response body and returns an *APIError.
func parseAPIError(resp *http.Response) error {
	var apiErr APIError
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("unexpected status %d", resp.StatusCode),
		}
	}
	apiErr.StatusCode = resp.StatusCode
	return &apiErr
}
```

### Users Sub-Client

```go
package client

import (
	"context"
	"fmt"
)

// UsersService handles user-related API calls.
type UsersService struct {
	client *Client
}

// User represents a user resource.
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// CreateUserInput is the input for creating a user.
type CreateUserInput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UpdateUserInput is the input for updating a user.
type UpdateUserInput struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// ListOptions controls pagination and filtering for list operations.
type ListOptions struct {
	Page    int
	PerPage int
	Filter  string
}

// Get retrieves a user by ID.
func (s *UsersService) Get(ctx context.Context, id string) (*User, error) {
	req, err := s.client.newRequest(ctx, "GET", fmt.Sprintf("users/%s", id), nil)
	if err != nil {
		return nil, err
	}

	var user User
	if err := s.client.do(req, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// List retrieves all users with optional filtering.
func (s *UsersService) List(ctx context.Context, opts *ListOptions) ([]*User, error) {
	path := "users"
	if opts != nil {
		params := map[string]string{}
		if opts.Page > 0 {
			params["page"] = fmt.Sprintf("%d", opts.Page)
		}
		if opts.PerPage > 0 {
			params["per_page"] = fmt.Sprintf("%d", opts.PerPage)
		}
		if opts.Filter != "" {
			params["filter"] = opts.Filter
		}
		path = addQueryParams(path, params)
	}

	req, err := s.client.newRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var users []*User
	if err := s.client.do(req, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// Create creates a new user.
func (s *UsersService) Create(ctx context.Context, input CreateUserInput) (*User, error) {
	req, err := s.client.newRequest(ctx, "POST", "users", input)
	if err != nil {
		return nil, err
	}

	var user User
	if err := s.client.do(req, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// Update updates an existing user.
func (s *UsersService) Update(ctx context.Context, id string, input UpdateUserInput) (*User, error) {
	req, err := s.client.newRequest(ctx, "PUT", fmt.Sprintf("users/%s", id), input)
	if err != nil {
		return nil, err
	}

	var user User
	if err := s.client.do(req, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// Delete removes a user by ID.
func (s *UsersService) Delete(ctx context.Context, id string) error {
	req, err := s.client.newRequest(ctx, "DELETE", fmt.Sprintf("users/%s", id), nil)
	if err != nil {
		return err
	}
	return s.client.do(req, nil)
}
```

### Projects Sub-Client

```go
package client

import (
	"context"
	"fmt"
)

// ProjectsService handles project-related API calls.
type ProjectsService struct {
	client *Client
}

// Project represents a project resource.
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	OwnerID     string `json:"owner_id"`
	Description string `json:"description"`
}

// CreateProjectInput is the input for creating a project.
type CreateProjectInput struct {
	Name        string `json:"name"`
	OwnerID     string `json:"owner_id"`
	Description string `json:"description,omitempty"`
}

// List retrieves all projects.
func (s *ProjectsService) List(ctx context.Context, opts *ListOptions) ([]*Project, error) {
	path := "projects"
	if opts != nil {
		params := map[string]string{}
		if opts.Page > 0 {
			params["page"] = fmt.Sprintf("%d", opts.Page)
		}
		if opts.PerPage > 0 {
			params["per_page"] = fmt.Sprintf("%d", opts.PerPage)
		}
		path = addQueryParams(path, params)
	}

	req, err := s.client.newRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var projects []*Project
	if err := s.client.do(req, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// Get retrieves a project by ID.
func (s *ProjectsService) Get(ctx context.Context, id string) (*Project, error) {
	req, err := s.client.newRequest(ctx, "GET", fmt.Sprintf("projects/%s", id), nil)
	if err != nil {
		return nil, err
	}

	var project Project
	if err := s.client.do(req, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// Create creates a new project.
func (s *ProjectsService) Create(ctx context.Context, input CreateProjectInput) (*Project, error) {
	req, err := s.client.newRequest(ctx, "POST", "projects", input)
	if err != nil {
		return nil, err
	}

	var project Project
	if err := s.client.do(req, &project); err != nil {
		return nil, err
	}
	return &project, nil
}
```

### Complete Usage Example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"myapp/pkg/client"
)

func main() {
	c := client.New(
		client.WithAuthToken(os.Getenv("MYAPP_TOKEN")),
		client.WithTimeout(10*time.Second),
		client.WithLogger(slog.Default()),
		client.WithBaseURL("https://api.staging.myapp.com"),
	)

	ctx := context.Background()

	// Create a user.
	user, err := c.Users.Create(ctx, client.CreateUserInput{
		Name:  "Alice",
		Email: "alice@example.com",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created user: %s\n", user.ID)

	// List users with pagination.
	users, err := c.Users.List(ctx, &client.ListOptions{
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, u := range users {
		fmt.Printf("  %s: %s (%s)\n", u.ID, u.Name, u.Email)
	}

	// Create a project.
	project, err := c.Projects.Create(ctx, client.CreateProjectInput{
		Name:    "My Project",
		OwnerID: user.ID,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created project: %s\n", project.ID)

	// Error handling.
	_, err = c.Users.Get(ctx, "nonexistent")
	if client.IsNotFound(err) {
		fmt.Println("User not found (expected)")
	}
}
```

## Benefits

1. **Type Safety**: All request inputs and response types are Go structs with JSON tags, catching errors at compile time.
2. **Discoverability**: Resource grouping (`c.Users.Get()`) makes the API surface easy to explore with IDE autocomplete.
3. **Flexible Configuration**: Functional options allow sensible defaults while supporting full customization.
4. **Testability**: Consumers can use `httptest.NewServer` to mock the API, or inject a custom `*http.Client` with a round-tripper.
5. **Consistent Error Handling**: `APIError` with helper predicates (`IsNotFound`, `IsConflict`) gives consumers a clean error-checking API.

## Best Practices

- Always accept `context.Context` as the first parameter of every method. This enables caller-controlled timeouts and cancellation.
- Use `WithBaseURL` for testing so consumers can point the client at `httptest.NewServer`.
- Return pointer types (`*User`, not `User`) from methods so callers can distinguish "zero value" from "not found."
- Use `omitempty` JSON tags on update input fields so zero values are not sent as patches.
- Export types and methods that consumers need; keep internal helpers unexported.
- Document each method's HTTP semantics (which HTTP method, which status codes) in godoc comments.

## Anti-Patterns

### Exposing the HTTP Client Directly

**Bad**: Letting consumers call `client.HTTPClient.Do()` to bypass the typed methods.

**Good**: Keep `httpClient` unexported. Provide `WithHTTPClient` for transport-level customization only.

### Global Client Variable

**Bad**: A package-level `var DefaultClient = New()` that stores auth tokens in global state.

**Good**: Always require explicit construction with `New(opts...)`.

### Returning Raw *http.Response

**Bad**: Returning `*http.Response` from methods, forcing consumers to handle status codes and JSON decoding.

**Good**: Decode the response inside the method and return typed Go values.

### Ignoring Context

**Bad**: Methods that do not accept `context.Context`, making it impossible for callers to set timeouts.

```go
// Bad: no context
func (s *UsersService) Get(id string) (*User, error) { ... }
```

**Good**: Always accept context as the first parameter.

```go
// Good: context-aware
func (s *UsersService) Get(ctx context.Context, id string) (*User, error) { ... }
```

## Related Patterns

- **[adapter-rest](./core-sdk-go.adapter-rest.md)**: The server-side counterpart that this client calls.
- **[adapter-base](./core-sdk-go.adapter-base.md)**: The lifecycle interface (client adapters may not need Start/Stop but follow the same structure).

## Testing

### Unit Testing with httptest

```go
package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"myapp/pkg/client"
)

func TestUsersGet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/users/usr_123" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatal("missing auth header")
		}

		json.NewEncoder(w).Encode(client.User{
			ID:    "usr_123",
			Name:  "Alice",
			Email: "alice@example.com",
		})
	}))
	defer ts.Close()

	c := client.New(
		client.WithBaseURL(ts.URL),
		client.WithAuthToken("test-token"),
	)

	user, err := c.Users.Get(context.Background(), "usr_123")
	if err != nil {
		t.Fatal(err)
	}
	if user.Name != "Alice" {
		t.Fatalf("expected Alice, got %s", user.Name)
	}
}
```

### Testing Error Responses

```go
func TestUsersGet_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"code":    404,
			"message": "user not found",
		})
	}))
	defer ts.Close()

	c := client.New(client.WithBaseURL(ts.URL))

	_, err := c.Users.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	if !client.IsNotFound(err) {
		t.Fatalf("expected not-found error, got: %v", err)
	}
}
```

### Testing with a Custom RoundTripper

```go
package client_test

import (
	"context"
	"net/http"
	"testing"

	"myapp/pkg/client"
)

type recordingTransport struct {
	requests []*http.Request
	inner    http.RoundTripper
}

func (t *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.requests = append(t.requests, req)
	return t.inner.RoundTrip(req)
}

func TestClient_SetsUserAgent(t *testing.T) {
	transport := &recordingTransport{inner: http.DefaultTransport}
	hc := &http.Client{Transport: transport}

	// Point at a real test server that returns 200 OK
	// (omitted for brevity)

	c := client.New(
		client.WithHTTPClient(hc),
		client.WithUserAgent("custom-agent/2.0"),
	)

	c.Users.List(context.Background(), nil)

	if len(transport.requests) == 0 {
		t.Fatal("no requests recorded")
	}
	ua := transport.requests[0].Header.Get("User-Agent")
	if ua != "custom-agent/2.0" {
		t.Fatalf("expected custom user agent, got %s", ua)
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
