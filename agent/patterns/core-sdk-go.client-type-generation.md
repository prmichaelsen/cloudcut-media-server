# Pattern: Client Type Generation

**Namespace**: core-sdk-go
**Category**: Client SDK
**Created**: 2026-03-17
**Status**: Active

---

## Overview

Defines the workflow for generating Go types, client interfaces, and server stubs from an OpenAPI specification. Code generation ensures the Go SDK stays in sync with the API contract. This pattern covers tool selection, configuration, automation with `go:generate`, and strategies for extending generated code without modifying generated files.

---

## Problem

Manually writing Go types to match an API specification is error-prone and drifts over time. When the API adds a field, changes a type, or introduces a new endpoint, the Go SDK must be updated in lockstep. Manual updates are slow, inconsistent, and miss edge cases like nullable fields, enums, and nested objects.

---

## Solution

Use an OpenAPI-to-Go code generator to produce types, client interfaces, and optionally server stubs from the API specification. Automate generation with `go:generate` directives so that `go generate ./...` rebuilds all generated code. Extend generated code through separate files that add methods to generated types or wrap generated clients -- never edit generated files directly.

---

## Implementation

### Tool Comparison

| Tool | Strengths | Weaknesses | Best For |
|------|-----------|------------|----------|
| **oapi-codegen** | Go-native, lightweight, chi/echo/gin/stdlib support, excellent type generation | Fewer features than openapi-generator | Go-only projects, clean type generation |
| **openapi-generator** | Multi-language, extensive customization via templates | Heavy (Java/JVM), verbose output, requires JVM | Polyglot orgs, heavy customization |
| **go-swagger** | Mature, validates specs, generates full server scaffolds | Only supports Swagger 2.0 (not OpenAPI 3.x) | Legacy Swagger 2.0 APIs |

**Recommendation**: Use `oapi-codegen` for Go projects. It produces idiomatic Go, has no JVM dependency, and integrates cleanly with `go:generate`.

### Installing oapi-codegen

```bash
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
```

### OpenAPI Specification

Start with a spec file. This can live in the repo or be fetched from a URL.

```yaml
# api/openapi.yaml
openapi: "3.0.3"
info:
  title: Example API
  version: "1.0.0"
paths:
  /users:
    get:
      operationId: listUsers
      summary: List all users
      parameters:
        - name: page
          in: query
          schema:
            type: integer
            default: 1
        - name: per_page
          in: query
          schema:
            type: integer
            default: 20
        - name: role
          in: query
          schema:
            type: string
      responses:
        "200":
          description: A list of users
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/ListUsersResponse"
    post:
      operationId: createUser
      summary: Create a user
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/CreateUserRequest"
      responses:
        "201":
          description: The created user
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/User"
  /users/{id}:
    get:
      operationId: getUser
      summary: Get a user by ID
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: The user
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/User"
        "404":
          description: User not found
    patch:
      operationId: updateUser
      summary: Update a user
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/UpdateUserRequest"
      responses:
        "200":
          description: The updated user
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/User"
    delete:
      operationId: deleteUser
      summary: Delete a user
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        "204":
          description: User deleted

components:
  schemas:
    User:
      type: object
      required: [id, name, email, role, created_at, updated_at]
      properties:
        id:
          type: string
        name:
          type: string
        email:
          type: string
          format: email
        role:
          type: string
          enum: [admin, editor, viewer]
        created_at:
          type: string
          format: date-time
        updated_at:
          type: string
          format: date-time

    CreateUserRequest:
      type: object
      required: [name, email]
      properties:
        name:
          type: string
        email:
          type: string
          format: email
        role:
          type: string
          enum: [admin, editor, viewer]

    UpdateUserRequest:
      type: object
      properties:
        name:
          type: string
        email:
          type: string
          format: email
        role:
          type: string
          enum: [admin, editor, viewer]

    ListUsersResponse:
      type: object
      required: [users, total_count, page, per_page]
      properties:
        users:
          type: array
          items:
            $ref: "#/components/schemas/User"
        total_count:
          type: integer
        page:
          type: integer
        per_page:
          type: integer
```

### oapi-codegen Configuration

Create a config file that controls what gets generated and where.

```yaml
# api/oapi-codegen.yaml
package: api
output: api/gen.go
generate:
  models: true
  client: true
  # Set to true if you also want server interface stubs:
  # chi-server: true
  # echo-server: true
  # std-http-server: true
output-options:
  skip-prune: false
  skip-fmt: false
```

### go:generate Directive

Place the directive in a Go file next to the config. This file exists solely to trigger generation.

```go
// api/generate.go
package api

//go:generate oapi-codegen --config oapi-codegen.yaml openapi.yaml
```

Run generation:

```bash
go generate ./api/...
```

### Generated Output (Example)

The generator produces types and a client interface. Here is a representative sample of what `oapi-codegen` outputs (simplified for clarity):

```go
// api/gen.go -- THIS FILE IS GENERATED. DO NOT EDIT.
package api

import (
	"time"
)

// UserRole defines the enum for user roles.
type UserRole string

const (
	UserRoleAdmin  UserRole = "admin"
	UserRoleEditor UserRole = "editor"
	UserRoleViewer UserRole = "viewer"
)

// User represents a user resource.
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      UserRole  `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateUserRequest is the request body for creating a user.
type CreateUserRequest struct {
	Name  string    `json:"name"`
	Email string    `json:"email"`
	Role  *UserRole `json:"role,omitempty"`
}

// UpdateUserRequest is the request body for updating a user.
type UpdateUserRequest struct {
	Name  *string   `json:"name,omitempty"`
	Email *string   `json:"email,omitempty"`
	Role  *UserRole `json:"role,omitempty"`
}

// ListUsersResponse wraps a paginated user list.
type ListUsersResponse struct {
	Users      []User `json:"users"`
	TotalCount int    `json:"total_count"`
	Page       int    `json:"page"`
	PerPage    int    `json:"per_page"`
}

// ListUsersParams holds query parameters for the list endpoint.
type ListUsersParams struct {
	Page    *int    `form:"page,omitempty" json:"page,omitempty"`
	PerPage *int    `form:"per_page,omitempty" json:"per_page,omitempty"`
	Role    *string `form:"role,omitempty" json:"role,omitempty"`
}
```

### Extending Generated Code

Never modify `gen.go`. Instead, create separate files in the same package that add methods or helper functions.

```go
// api/user_helpers.go
package api

import "fmt"

// DisplayName returns a formatted display string for the user.
func (u *User) DisplayName() string {
	return fmt.Sprintf("%s (%s)", u.Name, u.Role)
}

// IsAdmin returns true if the user has the admin role.
func (u *User) IsAdmin() bool {
	return u.Role == UserRoleAdmin
}

// Validate checks that required fields are present in a CreateUserRequest.
func (r *CreateUserRequest) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	if r.Email == "" {
		return fmt.Errorf("email is required")
	}
	return nil
}
```

### Wrapping the Generated Client

If you need to customize the generated client behavior, wrap it rather than editing it.

```go
// api/client_wrapper.go
package api

import (
	"context"
	"fmt"
	"net/http"
)

// ClientWrapper adds custom behavior around the generated client.
type ClientWrapper struct {
	inner *ClientWithResponses // Generated by oapi-codegen
}

// NewClientWrapper creates a wrapper around the generated client.
func NewClientWrapper(serverURL string, httpClient *http.Client) (*ClientWrapper, error) {
	c, err := NewClientWithResponses(serverURL,
		WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, fmt.Errorf("creating generated client: %w", err)
	}
	return &ClientWrapper{inner: c}, nil
}

// GetUser wraps the generated GetUser call with typed error handling.
func (w *ClientWrapper) GetUser(ctx context.Context, id string) (*User, error) {
	resp, err := w.inner.GetUserWithResponse(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetching user %s: %w", id, err)
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("user %s not found (status %d)", id, resp.StatusCode())
	}
	return resp.JSON200, nil
}

// ListUsers wraps the generated ListUsers call.
func (w *ClientWrapper) ListUsers(ctx context.Context, params *ListUsersParams) (*ListUsersResponse, error) {
	resp, err := w.inner.ListUsersWithResponse(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode())
	}
	return resp.JSON200, nil
}
```

### Complete Workflow

```
1. Author or update api/openapi.yaml
         |
         v
2. Run: go generate ./api/...
         |
         v
3. oapi-codegen reads openapi.yaml + oapi-codegen.yaml
         |
         v
4. Generates api/gen.go (types, client, optionally server stubs)
         |
         v
5. Extension files (api/user_helpers.go, api/client_wrapper.go) compile
   alongside generated code without modification
         |
         v
6. Service clients and app client import api package types
```

### Project Structure

```
myapp/
├── api/
│   ├── openapi.yaml          # OpenAPI specification (source of truth)
│   ├── oapi-codegen.yaml     # Generator configuration
│   ├── generate.go           # go:generate directive
│   ├── gen.go                # GENERATED -- do not edit
│   ├── user_helpers.go       # Hand-written extensions to generated types
│   └── client_wrapper.go     # Hand-written wrapper around generated client
├── sdk/
│   ├── client.go             # AppClient using api package types
│   └── options.go            # Functional options
└── go.mod
```

### Makefile Integration

```makefile
.PHONY: generate
generate:
	go generate ./...

.PHONY: generate-check
generate-check: generate
	@git diff --exit-code api/gen.go || \
		(echo "Generated code is out of date. Run 'make generate' and commit." && exit 1)
```

### CI Integration

Add a step to CI that verifies generated code is up to date:

```yaml
# .github/workflows/ci.yaml (relevant step)
- name: Check generated code
  run: |
    go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
    go generate ./...
    git diff --exit-code api/gen.go
```

---

## Benefits

1. **API contract as source of truth** -- types are derived from the spec, not hand-written
2. **Drift detection** -- CI catches when generated code is out of date
3. **Enum safety** -- OpenAPI enums become Go typed constants
4. **Nullable field handling** -- optional fields generate as pointers automatically
5. **Extensible** -- add methods to generated types in separate files without editing `gen.go`
6. **Reproducible** -- `go generate` rebuilds the same output from the same spec every time

---

## Best Practices

**Do commit generated code to the repository:**
```
# Generated files should be committed so that:
# - Builds don't require the generator tool
# - PRs show diffs in generated code for review
# - CI can verify freshness
```

**Do use a `.gitattributes` to mark generated files:**
```
# .gitattributes
api/gen.go linguist-generated=true
```

**Do pin the generator version:**
```bash
# In your project's tools.go or Makefile
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.0
```

**Do add a CI check for generated code freshness:**
```bash
go generate ./...
git diff --exit-code api/gen.go
```

**Do separate generated and hand-written code in the same package:**
```
api/
├── gen.go              # Generated -- never edit
├── user_helpers.go     # Hand-written extensions
└── client_wrapper.go   # Hand-written wrapper
```

---

## Anti-Patterns

**Don't edit generated files:**
```go
// BAD: Editing gen.go -- your changes will be overwritten on next generate
// api/gen.go
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// ADDED BY HAND: custom field <-- will be lost!
	DisplayName string `json:"display_name"`
}

// GOOD: Add methods in a separate file
// api/user_helpers.go
func (u *User) DisplayName() string {
	return u.Name
}
```

**Don't skip committing generated code:**
```
# BAD: .gitignore
api/gen.go   # Now builds require the generator tool

# GOOD: Commit gen.go so builds are self-contained
```

**Don't generate code in the wrong package:**
```yaml
# BAD: Generating into the main package
package: main
output: main_gen.go

# GOOD: Dedicated api package
package: api
output: api/gen.go
```

**Don't use openapi-generator just for Go types:**
```bash
# BAD: Pulling in JVM for simple type generation
docker run openapitools/openapi-generator-cli generate \
  -i openapi.yaml -g go -o ./api

# GOOD: Use the Go-native tool
oapi-codegen --config oapi-codegen.yaml openapi.yaml
```

**Don't write types by hand when a spec exists:**
```go
// BAD: Hand-written types that may drift from the API
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Missing: email, role, created_at, updated_at
}

// GOOD: Generate from the spec to stay in sync
//go:generate oapi-codegen --config oapi-codegen.yaml openapi.yaml
```

---

## Related Patterns

- [Service Client](core-sdk-go.client-svc.md) -- uses generated types for request/response bodies
- [Application Client](core-sdk-go.client-app.md) -- orchestrates service clients that use generated types
- [Client HTTP Transport](core-sdk-go.client-http-transport.md) -- transport chain used by the generated or wrapped client

---

## Testing

### Test: Generated Types Compile

The simplest test is that the package compiles. If the spec changes in a way that breaks existing code, `go build` will catch it.

```bash
go build ./api/...
```

### Test: Extension Methods

```go
package api_test

import (
	"testing"

	"myapp/api"
)

func TestUser_DisplayName(t *testing.T) {
	u := api.User{
		Name: "Alice",
		Role: api.UserRoleAdmin,
	}
	want := "Alice (admin)"
	if got := u.DisplayName(); got != want {
		t.Errorf("DisplayName() = %q, want %q", got, want)
	}
}

func TestUser_IsAdmin(t *testing.T) {
	tests := []struct {
		role api.UserRole
		want bool
	}{
		{api.UserRoleAdmin, true},
		{api.UserRoleEditor, false},
		{api.UserRoleViewer, false},
	}
	for _, tt := range tests {
		u := api.User{Role: tt.role}
		if got := u.IsAdmin(); got != tt.want {
			t.Errorf("IsAdmin() for role %s = %v, want %v", tt.role, got, tt.want)
		}
	}
}

func TestCreateUserRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     api.CreateUserRequest
		wantErr bool
	}{
		{
			name:    "valid",
			req:     api.CreateUserRequest{Name: "Alice", Email: "alice@example.com"},
			wantErr: false,
		},
		{
			name:    "missing name",
			req:     api.CreateUserRequest{Email: "alice@example.com"},
			wantErr: true,
		},
		{
			name:    "missing email",
			req:     api.CreateUserRequest{Name: "Alice"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
```

### Test: Generated Code Freshness (CI)

```go
package api_test

import (
	"os/exec"
	"testing"
)

func TestGeneratedCodeIsFresh(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping generation check in short mode")
	}

	// Run go generate.
	cmd := exec.Command("go", "generate", "./...")
	cmd.Dir = ".."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go generate failed: %v\n%s", err, out)
	}

	// Check for uncommitted changes to generated files.
	cmd = exec.Command("git", "diff", "--exit-code", "api/gen.go")
	cmd.Dir = ".."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated code is stale. Run 'go generate ./...' and commit.\n%s", out)
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
