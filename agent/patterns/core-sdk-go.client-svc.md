# Pattern: Service Client

**Namespace**: core-sdk-go
**Category**: Client SDK
**Created**: 2026-03-17
**Status**: Active

---

## Overview

Defines a service client struct that maps 1:1 to a REST API resource. Each endpoint gets exactly one receiver method on the service client. The service client receives a pre-configured `*http.Client` (with the transport chain from `client-http-transport`) and handles URL construction, query parameter encoding, request serialization, and response deserialization. Every method returns `(T, error)`.

---

## Problem

When consuming a REST API, callers need to construct URLs, encode query parameters, serialize request bodies, send HTTP requests, check status codes, and deserialize responses. Repeating this for every endpoint across the codebase leads to boilerplate duplication, inconsistent error handling, and tight coupling to HTTP details in business logic.

---

## Solution

Create a typed service client struct per API resource. The struct holds a base URL and an `*http.Client`. Each REST endpoint maps to a single receiver method that encapsulates all HTTP mechanics. Request and response types are plain Go structs with JSON tags. The caller works with domain types and errors, never with raw HTTP.

---

## Implementation

### Request and Response Types

```go
package users

import "time"

// User represents a user resource returned by the API.
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateUserRequest is the payload for creating a new user.
type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role,omitempty"`
}

// UpdateUserRequest is the payload for updating an existing user.
type UpdateUserRequest struct {
	Name  *string `json:"name,omitempty"`
	Email *string `json:"email,omitempty"`
	Role  *string `json:"role,omitempty"`
}

// ListUsersParams holds query parameters for listing users.
type ListUsersParams struct {
	Page    int    `url:"page,omitempty"`
	PerPage int    `url:"per_page,omitempty"`
	Role    string `url:"role,omitempty"`
	Search  string `url:"search,omitempty"`
}

// ListUsersResponse wraps a paginated list of users.
type ListUsersResponse struct {
	Users      []User `json:"users"`
	TotalCount int    `json:"total_count"`
	Page       int    `json:"page"`
	PerPage    int    `json:"per_page"`
}
```

### Service Client Struct

```go
package users

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// UserService provides methods for the /users API resource.
type UserService struct {
	client  *http.Client
	baseURL string
}

// NewUserService creates a UserService.
// The provided http.Client should already have the transport chain configured
// (auth, retry, logging).
func NewUserService(client *http.Client, baseURL string) *UserService {
	return &UserService{
		client:  client,
		baseURL: baseURL,
	}
}

// Get retrieves a single user by ID.
// GET /users/{id}
func (s *UserService) Get(ctx context.Context, id string) (*User, error) {
	reqURL := fmt.Sprintf("%s/users/%s", s.baseURL, url.PathEscape(id))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("users.Get: creating request: %w", err)
	}

	var user User
	if err := s.do(req, &user); err != nil {
		return nil, fmt.Errorf("users.Get: %w", err)
	}
	return &user, nil
}

// List retrieves a paginated list of users.
// GET /users?page=1&per_page=20&role=admin
func (s *UserService) List(ctx context.Context, params ListUsersParams) (*ListUsersResponse, error) {
	reqURL := fmt.Sprintf("%s/users", s.baseURL)

	q := url.Values{}
	if params.Page > 0 {
		q.Set("page", strconv.Itoa(params.Page))
	}
	if params.PerPage > 0 {
		q.Set("per_page", strconv.Itoa(params.PerPage))
	}
	if params.Role != "" {
		q.Set("role", params.Role)
	}
	if params.Search != "" {
		q.Set("search", params.Search)
	}
	if encoded := q.Encode(); encoded != "" {
		reqURL += "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("users.List: creating request: %w", err)
	}

	var result ListUsersResponse
	if err := s.do(req, &result); err != nil {
		return nil, fmt.Errorf("users.List: %w", err)
	}
	return &result, nil
}

// Create creates a new user.
// POST /users
func (s *UserService) Create(ctx context.Context, input CreateUserRequest) (*User, error) {
	reqURL := fmt.Sprintf("%s/users", s.baseURL)

	body, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("users.Create: marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("users.Create: creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	var user User
	if err := s.do(req, &user); err != nil {
		return nil, fmt.Errorf("users.Create: %w", err)
	}
	return &user, nil
}

// Update patches an existing user.
// PATCH /users/{id}
func (s *UserService) Update(ctx context.Context, id string, input UpdateUserRequest) (*User, error) {
	reqURL := fmt.Sprintf("%s/users/%s", s.baseURL, url.PathEscape(id))

	body, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("users.Update: marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("users.Update: creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	var user User
	if err := s.do(req, &user); err != nil {
		return nil, fmt.Errorf("users.Update: %w", err)
	}
	return &user, nil
}

// Delete removes a user by ID.
// DELETE /users/{id}
func (s *UserService) Delete(ctx context.Context, id string) error {
	reqURL := fmt.Sprintf("%s/users/%s", s.baseURL, url.PathEscape(id))

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
	if err != nil {
		return fmt.Errorf("users.Delete: creating request: %w", err)
	}

	if err := s.do(req, nil); err != nil {
		return fmt.Errorf("users.Delete: %w", err)
	}
	return nil
}

// do executes the request, checks the response, and decodes the body into dest.
// If dest is nil, the response body is discarded.
func (s *UserService) do(req *http.Request, dest interface{}) error {
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return err
	}

	if dest == nil {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	return nil
}

// checkResponse returns an error if the HTTP status code indicates failure.
func checkResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return &APIError{
		StatusCode: resp.StatusCode,
		Body:       string(body),
	}
}

// APIError represents a non-2xx HTTP response.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("api error (status %d): %s", e.StatusCode, e.Body)
}
```

### Constructor Wiring

The service client receives a fully configured `*http.Client` at construction time. This decouples the service client from transport concerns.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"myapp/sdk"
	"myapp/users"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// NewHTTPClient returns an *http.Client with auth, retry, and logging transports.
	httpClient := sdk.NewHTTPClient(os.Getenv("API_TOKEN"), logger)

	// Service client receives the configured http.Client.
	userSvc := users.NewUserService(httpClient, "https://api.example.com")

	ctx := context.Background()

	// Create a user.
	created, err := userSvc.Create(ctx, users.CreateUserRequest{
		Name:  "Alice",
		Email: "alice@example.com",
		Role:  "admin",
	})
	if err != nil {
		log.Fatalf("creating user: %v", err)
	}
	fmt.Printf("Created user: %s (%s)\n", created.Name, created.ID)

	// List users with pagination.
	result, err := userSvc.List(ctx, users.ListUsersParams{
		Page:    1,
		PerPage: 20,
		Role:    "admin",
	})
	if err != nil {
		log.Fatalf("listing users: %v", err)
	}
	fmt.Printf("Found %d users (page %d)\n", result.TotalCount, result.Page)

	// Get a single user.
	user, err := userSvc.Get(ctx, created.ID)
	if err != nil {
		log.Fatalf("getting user: %v", err)
	}
	fmt.Printf("User: %s (%s)\n", user.Name, user.Email)

	// Update a user.
	newName := "Alice Smith"
	updated, err := userSvc.Update(ctx, created.ID, users.UpdateUserRequest{
		Name: &newName,
	})
	if err != nil {
		log.Fatalf("updating user: %v", err)
	}
	fmt.Printf("Updated user: %s\n", updated.Name)

	// Delete a user.
	if err := userSvc.Delete(ctx, created.ID); err != nil {
		log.Fatalf("deleting user: %v", err)
	}
	fmt.Println("User deleted")
}
```

---

## Benefits

1. **1:1 mapping to REST endpoints** -- easy to find the method for any API call
2. **Type safety** -- request and response types catch errors at compile time
3. **Encapsulated HTTP mechanics** -- callers never construct URLs or parse JSON
4. **Consistent error handling** -- the `do` helper ensures every method checks responses the same way
5. **Testable** -- inject an `*http.Client` with a mock transport or use `httptest.Server`
6. **Discoverable** -- method signatures serve as API documentation

---

## Best Practices

**Do use `http.NewRequestWithContext` for every request:**
```go
req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
```

**Do escape path parameters:**
```go
reqURL := fmt.Sprintf("%s/users/%s", s.baseURL, url.PathEscape(id))
```

**Do use pointer fields for optional update payloads:**
```go
type UpdateUserRequest struct {
	Name  *string `json:"name,omitempty"`  // nil means "don't update"
	Email *string `json:"email,omitempty"`
}
```

**Do extract a shared `do` method for request execution:**
```go
func (s *UserService) do(req *http.Request, dest interface{}) error {
	// Execute, check, decode -- in one place
}
```

**Do prefix errors with the method name for traceability:**
```go
return nil, fmt.Errorf("users.Get: %w", err)
```

---

## Anti-Patterns

**Don't create the http.Client inside the service client:**
```go
// BAD: Hard-coded transport, untestable
func NewUserService(baseURL string) *UserService {
	return &UserService{
		client:  &http.Client{}, // No auth, no retry, no logging
		baseURL: baseURL,
	}
}

// GOOD: Accept a configured *http.Client
func NewUserService(client *http.Client, baseURL string) *UserService {
	return &UserService{
		client:  client,
		baseURL: baseURL,
	}
}
```

**Don't mix business logic into service client methods:**
```go
// BAD: Validation belongs in the application layer
func (s *UserService) Create(ctx context.Context, input CreateUserRequest) (*User, error) {
	if !strings.Contains(input.Email, "@") {
		return nil, fmt.Errorf("invalid email") // Not the client's job
	}
	// ... HTTP call
}
```

**Don't return raw *http.Response from service methods:**
```go
// BAD: Leaks HTTP details to the caller
func (s *UserService) Get(ctx context.Context, id string) (*http.Response, error) { ... }

// GOOD: Return domain types
func (s *UserService) Get(ctx context.Context, id string) (*User, error) { ... }
```

**Don't hard-code query parameters:**
```go
// BAD: Not composable
func (s *UserService) ListAdmins(ctx context.Context) (*ListUsersResponse, error) {
	url := s.baseURL + "/users?role=admin&per_page=100"
	// ...
}

// GOOD: Use a params struct
func (s *UserService) List(ctx context.Context, params ListUsersParams) (*ListUsersResponse, error) {
	// Build query from params
}
```

---

## Related Patterns

- [Client HTTP Transport](core-sdk-go.client-http-transport.md) -- provides the `*http.Client` consumed by service clients
- [Application Client](core-sdk-go.client-app.md) -- orchestrates multiple service clients for compound operations
- [Client Type Generation](core-sdk-go.client-type-generation.md) -- auto-generates service client types from OpenAPI specs
- [Service Base](core-sdk-go.service-base.md) -- server-side service pattern (the other side of this client)

---

## Testing

### Unit Test with httptest.Server

```go
package users_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"myapp/users"
)

func TestUserService_Get(t *testing.T) {
	want := users.User{
		ID:    "usr_123",
		Name:  "Alice",
		Email: "alice@example.com",
		Role:  "admin",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/users/usr_123" {
			t.Errorf("path = %s, want /users/usr_123", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(want)
	}))
	defer server.Close()

	svc := users.NewUserService(server.Client(), server.URL)

	got, err := svc.Get(context.Background(), "usr_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != want.ID || got.Name != want.Name {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestUserService_List_WithParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("role") != "admin" {
			t.Errorf("role param = %q, want admin", r.URL.Query().Get("role"))
		}
		if r.URL.Query().Get("page") != "2" {
			t.Errorf("page param = %q, want 2", r.URL.Query().Get("page"))
		}
		resp := users.ListUsersResponse{
			Users:      []users.User{{ID: "usr_1", Name: "Alice"}},
			TotalCount: 50,
			Page:       2,
			PerPage:    20,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := users.NewUserService(server.Client(), server.URL)

	result, err := svc.List(context.Background(), users.ListUsersParams{
		Page: 2,
		Role: "admin",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalCount != 50 {
		t.Errorf("total = %d, want 50", result.TotalCount)
	}
}

func TestUserService_Create(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		var input users.CreateUserRequest
		json.NewDecoder(r.Body).Decode(&input)

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(users.User{
			ID:    "usr_new",
			Name:  input.Name,
			Email: input.Email,
			Role:  input.Role,
		})
	}))
	defer server.Close()

	svc := users.NewUserService(server.Client(), server.URL)

	user, err := svc.Create(context.Background(), users.CreateUserRequest{
		Name:  "Bob",
		Email: "bob@example.com",
		Role:  "viewer",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Name != "Bob" {
		t.Errorf("name = %q, want Bob", user.Name)
	}
}

func TestUserService_Get_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"user not found"}`))
	}))
	defer server.Close()

	svc := users.NewUserService(server.Client(), server.URL)

	_, err := svc.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}

	var apiErr *users.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("status = %d, want 404", apiErr.StatusCode)
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
