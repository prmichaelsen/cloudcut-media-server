# Pattern: REST Client SDK (Overview)

**Namespace**: core-sdk-go
**Category**: Client SDK
**Created**: 2026-03-17
**Status**: Active

---

## Overview

This is a **synthesis pattern** that describes how the four granular client SDK patterns fit together to form a complete, idiomatic Go REST client SDK. Each component is documented in detail in its own pattern file — this document explains the architecture, data flow, and relationships between them.

The Go REST Client SDK provides typed HTTP client wrappers that mirror REST API routes. Every method returns `(T, error)` — Go's idiomatic multi-return pattern. Errors are typed (`*APIError`) for programmatic handling.

---

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                      Consumer Code                            │
│  user, err := client.Users.Get(ctx, "user-123")              │
└────────────────────────┬─────────────────────────────────────┘
                         │
         ┌───────────────┴───────────────┐
         │                               │
┌────────▼─────────┐          ┌──────────▼──────────┐
│   Svc Client     │          │    App Client       │
│   (1:1 REST)     │          │   (Compound Ops)    │
│                  │          │                     │
│ Users.Get()      │          │ CreateUserWithTeam()│
│ Users.Create()   │          │ TransferOwnership() │
│ Projects.List()  │          │ OnboardUser()       │
└────────┬─────────┘          └──────────┬──────────┘
         │                               │
         └───────────────┬───────────────┘
                         │
              ┌──────────▼──────────┐
              │   http.Client with  │
              │   Transport Chain   │
              │                     │
              │  • AuthTransport    │
              │  • RetryTransport   │
              │  • LoggingTransport │
              └──────────┬──────────┘
                         │
              ┌──────────▼──────────┐
              │    (T, error)       │
              │                     │
              │  T = typed response │
              │  error = *APIError  │
              └─────────────────────┘
```

### Component Map

| Component | Pattern Document | Purpose |
|-----------|-----------------|---------|
| Transport Chain | [HTTP Transport](core-sdk-go.client-http-transport.md) | `http.RoundTripper` middleware: auth, retry, logging, error normalization |
| Svc Client | [Service Client](core-sdk-go.client-svc.md) | 1:1 REST route mirror — one method per endpoint |
| App Client | [App Client](core-sdk-go.client-app.md) | Compound use-case operations (multi-step workflows) |
| Type Generation | [Type Generation](core-sdk-go.client-type-generation.md) | OpenAPI → Go types via `oapi-codegen` |

---

## Data Flow

### Request Path

```
consumer calls client.Users.Get(ctx, "user-123")
  │
  ├─ UsersService.Get() builds path + query params
  │   └─ calls c.do(ctx, "GET", "/api/v1/users/user-123", nil, &user)
  │       │
  │       ├─ http.NewRequestWithContext(ctx, method, url, body)
  │       ├─ c.httpClient.Do(req)
  │       │   └─ Transport chain executes:
  │       │       LoggingTransport.RoundTrip()
  │       │         → RetryTransport.RoundTrip()
  │       │           → AuthTransport.RoundTrip()  (adds Bearer token)
  │       │             → http.DefaultTransport.RoundTrip()
  │       │
  │       ├─ checkResponse(resp)  → returns *APIError if status >= 400
  │       └─ json.Decode(resp.Body, &user)
  │
  └─ consumer receives (user, nil)
     OR (zero-value, &APIError{Code: "not_found", Status: 404, ...})
```

### Error Normalization

All failures — network errors, HTTP errors, auth errors — are normalized into a typed error:

```go
// APIError represents any error from the REST API.
type APIError struct {
    Code    string `json:"code"`    // "not_found", "unauthorized", "validation", etc.
    Message string `json:"message"` // Human-readable
    Status  int    `json:"status"`  // HTTP status (0 for network errors)
}

func (e *APIError) Error() string {
    return fmt.Sprintf("%s (status %d): %s", e.Code, e.Status, e.Message)
}

// Convenience checkers
func (e *APIError) IsNotFound() bool     { return e.Status == 404 }
func (e *APIError) IsConflict() bool     { return e.Status == 409 }
func (e *APIError) IsUnauthorized() bool { return e.Status == 401 }
func (e *APIError) IsRateLimited() bool  { return e.Status == 429 }
```

HTTP status → error code mapping:
- `400` → `bad_request` / `validation`
- `401` → `unauthorized`
- `403` → `forbidden`
- `404` → `not_found`
- `409` → `conflict`
- `429` → `rate_limited`
- `500` → `internal`
- Network failure → `network_error` (status: 0)

---

## Two Client Tiers

### Svc Client — Atomic Operations

1:1 mirror of REST routes. No business logic, no compound operations. Each method maps to exactly one HTTP request.

```go
client, err := sdk.NewClient("https://api.example.com",
    sdk.WithAuthToken(token),
    sdk.WithTimeout(10*time.Second),
)
if err != nil {
    log.Fatal(err)
}

// Each call = one HTTP request
user, err := client.Users.Get(ctx, "user-123")
users, err := client.Users.List(ctx, sdk.ListOptions{Page: 1, PerPage: 20})
project, err := client.Projects.Create(ctx, sdk.CreateProjectInput{Name: "my-project"})
```

**When to use**: Consumers who need full control over individual operations. Backend services, CLI tools, scripts.

See [Service Client Pattern](core-sdk-go.client-svc.md) for full details.

### App Client — Compound Operations

Use-case-oriented methods that compose multiple REST calls. Handles intermediate steps, rollbacks, and orchestration internally.

```go
app, err := sdk.NewAppClient("https://api.example.com",
    sdk.WithAuthToken(token),
)
if err != nil {
    log.Fatal(err)
}

// One call = multiple HTTP requests behind the scenes
result, err := app.CreateUserWithTeam(ctx, sdk.CreateUserWithTeamInput{
    User: sdk.CreateUserInput{Name: "Pat", Email: "pat@example.com"},
    Team: sdk.CreateTeamInput{Name: "Engineering"},
})
// Creates user, creates team, adds user to team — rolls back on failure
```

**When to use**: Applications with multi-step workflows. Reduces boilerplate in API handlers.

See [App Client Pattern](core-sdk-go.client-app.md) for full details.

---

## Auth Patterns

The transport chain supports auth via the `AuthTransport` `http.RoundTripper`:

### Static Token

```go
client, _ := sdk.NewClient(baseURL,
    sdk.WithAuthToken("my-bearer-token"),
)
```

### Token Callback (Dynamic)

```go
client, _ := sdk.NewClient(baseURL,
    sdk.WithTokenSource(func(ctx context.Context) (string, error) {
        // Resolve token from session, vault, etc.
        return myAuth.GetToken(ctx)
    }),
)
```

### No Auth

```go
client, _ := sdk.NewClient(baseURL) // No auth transport added
```

See [HTTP Transport Pattern](core-sdk-go.client-http-transport.md) for implementation details.

---

## Transport Chain Composition

Transports are composed inside-out — the outermost transport runs first:

```go
func buildTransport(token string, logger *slog.Logger) http.RoundTripper {
    base := http.DefaultTransport

    // Innermost: adds auth header
    auth := &AuthTransport{
        Token: token,
        Base:  base,
    }

    // Middle: retries on 429/5xx
    retry := &RetryTransport{
        Base:       auth,
        MaxRetries: 3,
        BaseDelay:  100 * time.Millisecond,
    }

    // Outermost: logs request/response
    logging := &LoggingTransport{
        Base:   retry,
        Logger: logger,
    }

    return logging
}

httpClient := &http.Client{
    Transport: buildTransport(token, logger),
    Timeout:   30 * time.Second,
}
```

---

## Type Safety with OpenAPI

Types are generated from OpenAPI specs using `oapi-codegen`:

```bash
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
```

```yaml
# oapi-codegen.yaml
package: api
output: internal/api/types.gen.go
generate:
  models: true
  client: true
```

```go
//go:generate oapi-codegen --config oapi-codegen.yaml docs/openapi.yaml
```

Generated types are used in hand-written client code:

```go
import "myproject/internal/api"

type UsersService struct {
    client *Client
}

func (s *UsersService) Create(ctx context.Context, input api.CreateUserRequest) (*api.User, error) {
    var user api.User
    err := s.client.do(ctx, "POST", "/api/v1/users", input, &user)
    return &user, err
}
```

See [Type Generation Pattern](core-sdk-go.client-type-generation.md) for full workflow.

---

## Module Exports

Go packages are organized by directory. A client SDK module might look like:

```
github.com/myorg/myproject-go/
├── client.go          // NewClient(), Client struct, functional options
├── users.go           // UsersService
├── projects.go        // ProjectsService
├── teams.go           // TeamsService
├── app.go             // NewAppClient(), compound operations
├── transport.go       // AuthTransport, RetryTransport, LoggingTransport
├── errors.go          // APIError
├── types.go           // Shared request/response types (or generated)
└── internal/
    └── api/
        └── types.gen.go  // OpenAPI-generated types
```

Consumers import the package:

```go
import sdk "github.com/myorg/myproject-go"

client, _ := sdk.NewClient("https://api.example.com",
    sdk.WithAuthToken(token),
)
```

---

## Adding a New Resource

To add a new resource (e.g., `notifications`):

1. **Create resource file** `notifications.go`:
   - Define `NotificationsService` struct with `*Client` field
   - Implement methods: `Get`, `List`, `Create`, `MarkRead`, etc.
   - Each method calls `c.client.do(ctx, method, path, input, &output)`

2. **Register in client** `client.go`:
   - Add `Notifications *NotificationsService` field to `Client` struct
   - Wire in `NewClient()`: `c.Notifications = &NotificationsService{client: c}`

3. **Generate types** (if OpenAPI spec exists):
   - Update the OpenAPI spec with new routes
   - Run `go generate ./...`
   - Reference generated types in the resource file

4. **Add tests** — use `httptest.NewServer` to mock HTTP responses

---

## Go vs TypeScript Differences

| Aspect | TypeScript core-sdk | Go core-sdk-go |
|--------|-------------------|----------------|
| Return type | `SdkResponse<T>` (never throws) | `(T, error)` (idiomatic Go) |
| Error checking | `if (response.error)` | `if err != nil` |
| Transport | Custom `fetch()` wrapper | `http.RoundTripper` interface |
| Auth | Callback or JWT generation | `AuthTransport` RoundTripper |
| Type generation | `openapi-typescript` | `oapi-codegen` |
| Browser guard | `assertServerSide()` | N/A (Go is server-side) |
| Package exports | `package.json` exports map | Go module import paths |
| Optional deps | Dynamic `import()` | Build tags or interface injection |

The key architectural insight: **Go's `http.RoundTripper` interface replaces the custom `HttpClient` class entirely.** Transport concerns (auth, retry, logging) are composed as middleware layers on the standard `http.Client`, which is more idiomatic and interoperable with the Go ecosystem.

---

## Anti-Patterns

### No Business Logic in Client Code
Client SDKs are **transport wrappers only**. Validation, caching, and transformation belong in server-side services.

### No Mixing Tiers
Svc Client = atomic 1:1 operations. App Client = compound workflows. Keep them separate.

### No Wrapping Svc Client in App Client
App Client calls HTTP directly (via the shared `*Client`), not through Svc Client methods. This keeps them decoupled for independent evolution.

### No Global http.Client
Always accept `*http.Client` or functional options. Never use `http.DefaultClient` in library code — it has no timeout and no transport customization.

### No Manual JSON in Callers
The `do()` helper handles marshaling/unmarshaling. Callers should never call `json.Marshal` or read `resp.Body` directly.

---

## Checklist for New Projects

- [ ] `client.go` — `NewClient()` with functional options, `do()` helper
- [ ] `transport.go` — `AuthTransport`, `RetryTransport`, `LoggingTransport`
- [ ] `errors.go` — `APIError` with status helpers
- [ ] Resource files — one per REST resource (`users.go`, `projects.go`, etc.)
- [ ] `app.go` — App client with compound operations (if needed)
- [ ] `internal/api/types.gen.go` — OpenAPI-generated types (if spec exists)
- [ ] `go:generate` directive for type regeneration
- [ ] Tests with `httptest.NewServer` for each resource
- [ ] `example_test.go` — Runnable examples for `go doc`

---

## Related Patterns (Detailed Docs)

- **[HTTP Transport](core-sdk-go.client-http-transport.md)** — `RoundTripper` middleware: auth, retry, logging, error normalization
- **[Service Client](core-sdk-go.client-svc.md)** — 1:1 resource wrappers, `do()` helper, request/response types
- **[App Client](core-sdk-go.client-app.md)** — Compound operations, rollback, concurrent fetches with errgroup
- **[Type Generation](core-sdk-go.client-type-generation.md)** — OpenAPI → Go workflow with `oapi-codegen`
- **[Service Error Handling](core-sdk-go.service-error-handling.md)** — Server-side error types that map to `APIError` codes
- **[Context Propagation](core-sdk-go.context-propagation.md)** — `context.Context` flows through all client methods

---

**Status**: Active
**Compatibility**: Go 1.21+
