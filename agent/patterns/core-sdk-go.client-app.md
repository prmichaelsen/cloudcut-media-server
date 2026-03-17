# Pattern: Application Client

**Namespace**: core-sdk-go
**Category**: Client SDK
**Created**: 2026-03-17
**Status**: Active

---

## Overview

Defines a high-level application client that wraps multiple service clients and provides compound operations. While service clients map 1:1 to REST endpoints, the application client exposes business-level methods that orchestrate calls across multiple services. Configuration uses the functional options pattern for a clean, extensible constructor.

---

## Problem

Real-world workflows rarely map to a single API call. Creating a user might also require creating a team membership, sending a welcome notification, and provisioning default settings. Scattering this orchestration across calling code leads to duplication and inconsistent multi-step flows. Callers also need a single entry point to the SDK rather than constructing multiple service clients individually.

---

## Solution

Create an `AppClient` struct that owns all service clients and exposes business-level methods. The constructor uses functional options for flexible configuration. Compound methods call multiple service clients in sequence (or concurrently where safe), handling partial failures and providing rollback where needed. Service clients remain accessible for direct use when the caller needs fine-grained control.

---

## Implementation

### Functional Options

```go
package sdk

import (
	"log/slog"
	"net/http"
	"time"

	"myapp/transport"
	"myapp/users"
	"myapp/teams"
	"myapp/notifications"
)

// Option configures the AppClient.
type Option func(*AppClient)

// WithHTTPClient sets a custom http.Client. If not provided, a default client
// with auth, retry, and logging transports is created from the token.
func WithHTTPClient(c *http.Client) Option {
	return func(a *AppClient) {
		a.httpClient = c
	}
}

// WithLogger sets the structured logger. Defaults to slog.Default().
func WithLogger(l *slog.Logger) Option {
	return func(a *AppClient) {
		a.logger = l
	}
}

// WithBaseURL overrides the API base URL. Defaults to https://api.example.com.
func WithBaseURL(url string) Option {
	return func(a *AppClient) {
		a.baseURL = url
	}
}

// WithTimeout sets the HTTP client timeout. Defaults to 30s.
// Ignored if WithHTTPClient is also provided.
func WithTimeout(d time.Duration) Option {
	return func(a *AppClient) {
		a.timeout = d
	}
}

// WithRetryConfig configures retry behavior.
// Ignored if WithHTTPClient is also provided.
func WithRetryConfig(maxRetries int, baseDelay, maxDelay time.Duration) Option {
	return func(a *AppClient) {
		a.maxRetries = maxRetries
		a.retryBaseDelay = baseDelay
		a.retryMaxDelay = maxDelay
	}
}
```

### AppClient Struct

```go
package sdk

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"myapp/notifications"
	"myapp/teams"
	"myapp/transport"
	"myapp/users"
)

// AppClient is the top-level SDK entry point. It provides business-level
// methods that orchestrate calls across multiple service clients.
type AppClient struct {
	// Exported service clients for direct access when needed.
	Users         *users.UserService
	Teams         *teams.TeamService
	Notifications *notifications.NotificationService

	// Internal configuration.
	httpClient     *http.Client
	logger         *slog.Logger
	baseURL        string
	timeout        time.Duration
	maxRetries     int
	retryBaseDelay time.Duration
	retryMaxDelay  time.Duration
}

// New creates an AppClient with the given API token and options.
func New(token string, opts ...Option) (*AppClient, error) {
	if token == "" {
		return nil, fmt.Errorf("sdk: token is required")
	}

	a := &AppClient{
		baseURL:        "https://api.example.com",
		logger:         slog.Default(),
		timeout:        30 * time.Second,
		maxRetries:     3,
		retryBaseDelay: 500 * time.Millisecond,
		retryMaxDelay:  30 * time.Second,
	}

	for _, opt := range opts {
		opt(a)
	}

	// Build the default HTTP client if none was provided.
	if a.httpClient == nil {
		auth := &transport.AuthTransport{
			Source: &transport.StaticTokenSource{AccessToken: token},
			Base:   http.DefaultTransport,
		}
		retry := &transport.RetryTransport{
			Base:       auth,
			MaxRetries: a.maxRetries,
			BaseDelay:  a.retryBaseDelay,
			MaxDelay:   a.retryMaxDelay,
		}
		logging := &transport.LoggingTransport{
			Base:   retry,
			Logger: a.logger,
		}
		a.httpClient = &http.Client{
			Transport: logging,
			Timeout:   a.timeout,
		}
	}

	// Initialize service clients.
	a.Users = users.NewUserService(a.httpClient, a.baseURL)
	a.Teams = teams.NewTeamService(a.httpClient, a.baseURL)
	a.Notifications = notifications.NewNotificationService(a.httpClient, a.baseURL)

	return a, nil
}
```

### Compound Operations

```go
package sdk

import (
	"context"
	"fmt"

	"myapp/notifications"
	"myapp/teams"
	"myapp/users"
)

// CreateUserWithTeamInput holds parameters for the compound operation.
type CreateUserWithTeamInput struct {
	UserName  string
	UserEmail string
	UserRole  string
	TeamID    string
	TeamRole  string // Role within the team (e.g., "member", "lead").
	SendWelcome bool
}

// CreateUserWithTeamResult holds all resources created by the compound operation.
type CreateUserWithTeamResult struct {
	User       *users.User
	Membership *teams.Membership
}

// CreateUserWithTeam creates a user, adds them to a team, and optionally sends
// a welcome notification. If any step fails after user creation, it attempts
// to clean up by deleting the created user.
func (a *AppClient) CreateUserWithTeam(ctx context.Context, input CreateUserWithTeamInput) (*CreateUserWithTeamResult, error) {
	// Step 1: Create the user.
	user, err := a.Users.Create(ctx, users.CreateUserRequest{
		Name:  input.UserName,
		Email: input.UserEmail,
		Role:  input.UserRole,
	})
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	// Step 2: Add user to team.
	membership, err := a.Teams.AddMember(ctx, input.TeamID, teams.AddMemberRequest{
		UserID: user.ID,
		Role:   input.TeamRole,
	})
	if err != nil {
		// Rollback: delete the user we just created.
		a.logger.Warn("rolling back user creation after team membership failure",
			"user_id", user.ID,
			"error", err,
		)
		if delErr := a.Users.Delete(ctx, user.ID); delErr != nil {
			a.logger.Error("rollback failed: could not delete user",
				"user_id", user.ID,
				"error", delErr,
			)
		}
		return nil, fmt.Errorf("adding user to team: %w", err)
	}

	// Step 3: Send welcome notification (best-effort, does not roll back).
	if input.SendWelcome {
		_, err := a.Notifications.Send(ctx, notifications.SendRequest{
			UserID:  user.ID,
			Type:    "welcome",
			Message: fmt.Sprintf("Welcome to the team, %s!", user.Name),
		})
		if err != nil {
			// Log but don't fail the operation -- notification is non-critical.
			a.logger.Warn("failed to send welcome notification",
				"user_id", user.ID,
				"error", err,
			)
		}
	}

	return &CreateUserWithTeamResult{
		User:       user,
		Membership: membership,
	}, nil
}
```

### Concurrent Operations

When steps are independent, run them concurrently with `errgroup`.

```go
package sdk

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"myapp/teams"
	"myapp/users"
)

// UserWithTeams holds a user and all their team memberships.
type UserWithTeams struct {
	User  *users.User
	Teams []teams.Team
}

// GetUserWithTeams fetches a user and their teams concurrently.
func (a *AppClient) GetUserWithTeams(ctx context.Context, userID string) (*UserWithTeams, error) {
	var (
		user      *users.User
		teamsList []teams.Team
	)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		user, err = a.Users.Get(ctx, userID)
		if err != nil {
			return fmt.Errorf("fetching user: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		var err error
		result, err := a.Teams.ListByUser(ctx, userID)
		if err != nil {
			return fmt.Errorf("fetching teams: %w", err)
		}
		teamsList = result.Teams
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &UserWithTeams{
		User:  user,
		Teams: teamsList,
	}, nil
}
```

### Usage

```go
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"myapp/sdk"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	// Create the app client with functional options.
	client, err := sdk.New(
		os.Getenv("API_TOKEN"),
		sdk.WithLogger(logger),
		sdk.WithBaseURL("https://api.staging.example.com"),
		sdk.WithTimeout(15*time.Second),
		sdk.WithRetryConfig(5, 1*time.Second, 60*time.Second),
	)
	if err != nil {
		log.Fatalf("creating client: %v", err)
	}

	ctx := context.Background()

	// Compound operation: create user with team membership.
	result, err := client.CreateUserWithTeam(ctx, sdk.CreateUserWithTeamInput{
		UserName:    "Alice",
		UserEmail:   "alice@example.com",
		UserRole:    "engineer",
		TeamID:      "team_platform",
		TeamRole:    "member",
		SendWelcome: true,
	})
	if err != nil {
		log.Fatalf("creating user with team: %v", err)
	}
	fmt.Printf("Created user %s in team (membership: %s)\n",
		result.User.ID, result.Membership.ID)

	// Direct service client access for fine-grained control.
	user, err := client.Users.Get(ctx, result.User.ID)
	if err != nil {
		log.Fatalf("getting user: %v", err)
	}
	fmt.Printf("User: %s (%s)\n", user.Name, user.Email)
}
```

---

## Benefits

1. **Single entry point** -- one constructor gives access to the entire SDK
2. **Business-level abstraction** -- compound methods express intent, not HTTP mechanics
3. **Rollback support** -- multi-step operations can clean up on partial failure
4. **Concurrent fetches** -- independent calls run in parallel with `errgroup`
5. **Extensible configuration** -- functional options allow adding new settings without breaking existing callers
6. **Direct access escape hatch** -- service clients remain exported for when callers need fine-grained control

---

## Best Practices

**Do use functional options for configuration:**
```go
client, err := sdk.New(token,
	sdk.WithBaseURL("https://api.staging.example.com"),
	sdk.WithTimeout(15 * time.Second),
)
```

**Do distinguish critical vs. best-effort steps:**
```go
// Critical: roll back on failure
membership, err := a.Teams.AddMember(ctx, teamID, req)
if err != nil {
	a.Users.Delete(ctx, user.ID) // Rollback
	return nil, err
}

// Best-effort: log but don't fail
if _, err := a.Notifications.Send(ctx, notif); err != nil {
	a.logger.Warn("notification failed", "error", err)
}
```

**Do use errgroup for concurrent independent calls:**
```go
g, ctx := errgroup.WithContext(ctx)
g.Go(func() error { /* fetch A */ return nil })
g.Go(func() error { /* fetch B */ return nil })
if err := g.Wait(); err != nil { return nil, err }
```

**Do keep service clients exported for direct access:**
```go
type AppClient struct {
	Users *users.UserService // Exported for direct use
	Teams *teams.TeamService
}
```

---

## Anti-Patterns

**Don't duplicate service client logic in compound methods:**
```go
// BAD: Reimplementing the HTTP call instead of using the service client
func (a *AppClient) CreateUserWithTeam(ctx context.Context, ...) error {
	req, _ := http.NewRequest("POST", a.baseURL+"/users", body) // Duplicated!
	resp, _ := a.httpClient.Do(req)
	// ...
}

// GOOD: Delegate to the service client
func (a *AppClient) CreateUserWithTeam(ctx context.Context, ...) error {
	user, err := a.Users.Create(ctx, input) // Use the service client
	// ...
}
```

**Don't silently swallow errors from critical steps:**
```go
// BAD: Ignoring team membership failure
func (a *AppClient) CreateUserWithTeam(ctx context.Context, ...) (*Result, error) {
	user, _ := a.Users.Create(ctx, userReq)
	a.Teams.AddMember(ctx, teamID, memberReq) // Error ignored!
	return &Result{User: user}, nil
}
```

**Don't use positional arguments for complex configuration:**
```go
// BAD: Positional args are unclear and fragile
func New(token, baseURL string, timeout time.Duration, maxRetries int, logger *slog.Logger) *AppClient

// GOOD: Functional options
func New(token string, opts ...Option) (*AppClient, error)
```

**Don't put transport concerns in compound methods:**
```go
// BAD: Auth logic in the app client
func (a *AppClient) CreateUserWithTeam(ctx context.Context, ...) error {
	req.Header.Set("Authorization", "Bearer "+a.token) // Transport's job!
}
```

---

## Related Patterns

- [Service Client](core-sdk-go.client-svc.md) -- the 1:1 REST endpoint clients that AppClient wraps
- [Client HTTP Transport](core-sdk-go.client-http-transport.md) -- transport chain consumed by all service clients
- [Client Type Generation](core-sdk-go.client-type-generation.md) -- auto-generating the types used by service and app clients
- [Service Base](core-sdk-go.service-base.md) -- server-side pattern for the services these clients call

---

## Testing

### Unit Test: Compound Operation

```go
package sdk_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"myapp/sdk"
	"myapp/teams"
	"myapp/users"
)

func TestAppClient_CreateUserWithTeam(t *testing.T) {
	mux := http.NewServeMux()

	// Mock user creation.
	mux.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		var input users.CreateUserRequest
		json.NewDecoder(r.Body).Decode(&input)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(users.User{
			ID:    "usr_new",
			Name:  input.Name,
			Email: input.Email,
			Role:  input.Role,
		})
	})

	// Mock team membership creation.
	mux.HandleFunc("POST /teams/team_eng/members", func(w http.ResponseWriter, r *http.Request) {
		var input teams.AddMemberRequest
		json.NewDecoder(r.Body).Decode(&input)
		json.NewEncoder(w).Encode(teams.Membership{
			ID:     "mem_new",
			UserID: input.UserID,
			TeamID: "team_eng",
			Role:   input.Role,
		})
	})

	// Mock notification (best-effort).
	mux.HandleFunc("POST /notifications", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"notif_1"}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	client, err := sdk.New("test-token",
		sdk.WithHTTPClient(server.Client()),
		sdk.WithBaseURL(server.URL),
		sdk.WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	result, err := client.CreateUserWithTeam(context.Background(), sdk.CreateUserWithTeamInput{
		UserName:    "Alice",
		UserEmail:   "alice@example.com",
		UserRole:    "engineer",
		TeamID:      "team_eng",
		TeamRole:    "member",
		SendWelcome: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.User.ID != "usr_new" {
		t.Errorf("user ID = %q, want usr_new", result.User.ID)
	}
	if result.Membership.ID != "mem_new" {
		t.Errorf("membership ID = %q, want mem_new", result.Membership.ID)
	}
}

func TestAppClient_CreateUserWithTeam_RollbackOnTeamFailure(t *testing.T) {
	var userDeleted bool
	mux := http.NewServeMux()

	mux.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(users.User{ID: "usr_rollback", Name: "Bob"})
	})

	mux.HandleFunc("POST /teams/team_eng/members", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"team service unavailable"}`))
	})

	mux.HandleFunc("DELETE /users/usr_rollback", func(w http.ResponseWriter, r *http.Request) {
		userDeleted = true
		w.WriteHeader(http.StatusNoContent)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client, _ := sdk.New("test-token",
		sdk.WithHTTPClient(server.Client()),
		sdk.WithBaseURL(server.URL),
	)

	_, err := client.CreateUserWithTeam(context.Background(), sdk.CreateUserWithTeamInput{
		UserName:  "Bob",
		UserEmail: "bob@example.com",
		TeamID:    "team_eng",
		TeamRole:  "member",
	})
	if err == nil {
		t.Fatal("expected error when team membership fails")
	}
	if !userDeleted {
		t.Error("expected user to be deleted as rollback")
	}
}
```

### Test: Functional Options

```go
package sdk_test

import (
	"testing"
	"time"

	"myapp/sdk"
)

func TestNew_RequiresToken(t *testing.T) {
	_, err := sdk.New("")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestNew_DefaultOptions(t *testing.T) {
	client, err := sdk.New("test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.Users == nil {
		t.Error("Users service client is nil")
	}
	if client.Teams == nil {
		t.Error("Teams service client is nil")
	}
}

func TestNew_WithCustomOptions(t *testing.T) {
	client, err := sdk.New("test-token",
		sdk.WithBaseURL("https://custom.api.com"),
		sdk.WithTimeout(5*time.Second),
		sdk.WithRetryConfig(1, 100*time.Millisecond, 1*time.Second),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.Users == nil {
		t.Error("Users service client is nil")
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
