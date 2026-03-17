# Pattern: MCP Adapter

**Namespace**: core-sdk-go
**Category**: Adapter Layer
**Created**: 2026-03-17
**Status**: Active

---

## Overview

The MCP (Model Context Protocol) Adapter pattern implements an MCP server in Go that exposes service-layer operations as tools for AI agents. It uses the `mcp-go` SDK to define tools with JSON Schema inputs, register handler functions, and serve over stdio transport. Like all adapters, it is a thin translation layer: it parses tool arguments, calls the service layer, and formats the response.

## Problem

Exposing application functionality to AI agents (such as Claude) requires implementing the MCP protocol: tool definitions with JSON Schema, request/response serialization, and transport management. Without a clear adapter pattern, tool handler logic becomes entangled with business logic, and adding new tools requires understanding protocol internals.

## Solution

Create an MCP adapter struct that owns the MCP server instance and its tool registrations. Each tool handler is a closure that captures the service dependency it needs, parses the JSON input, calls the service method, and returns a formatted result. The adapter implements the Base Adapter interface for consistent lifecycle management.

## Implementation

### Project Structure

```
internal/
  adapter/
    mcp/
      mcp.go           # Adapter struct, constructor, Start/Stop
      tools.go          # Tool definitions and registration
      handlers.go       # Tool handler functions
  service/
    user.go             # UserService interface + implementation
    project.go          # ProjectService interface + implementation
```

### Tool Definitions and Handlers

```go
package mcpadapter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"myapp/internal/service"
)

// registerTools adds all tool definitions and their handlers to the MCP server.
func registerTools(s *server.MCPServer, users service.UserService, projects service.ProjectService) {
	// --- User Tools ---

	s.AddTool(
		mcp.NewTool("get_user",
			mcp.WithDescription("Get a user by their ID"),
			mcp.WithString("user_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the user"),
			),
		),
		getUserHandler(users),
	)

	s.AddTool(
		mcp.NewTool("create_user",
			mcp.WithDescription("Create a new user"),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("The user's display name"),
			),
			mcp.WithString("email",
				mcp.Required(),
				mcp.Description("The user's email address"),
			),
		),
		createUserHandler(users),
	)

	s.AddTool(
		mcp.NewTool("list_users",
			mcp.WithDescription("List all users"),
			mcp.WithString("filter",
				mcp.Description("Optional filter string to match against user names"),
			),
		),
		listUsersHandler(users),
	)

	s.AddTool(
		mcp.NewTool("delete_user",
			mcp.WithDescription("Delete a user by their ID"),
			mcp.WithString("user_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the user to delete"),
			),
		),
		deleteUserHandler(users),
	)

	// --- Project Tools ---

	s.AddTool(
		mcp.NewTool("list_projects",
			mcp.WithDescription("List all projects, optionally filtered by owner"),
			mcp.WithString("owner_id",
				mcp.Description("Filter projects by owner user ID"),
			),
		),
		listProjectsHandler(projects),
	)

	s.AddTool(
		mcp.NewTool("create_project",
			mcp.WithDescription("Create a new project"),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("The project name"),
			),
			mcp.WithString("owner_id",
				mcp.Required(),
				mcp.Description("The user ID of the project owner"),
			),
			mcp.WithString("description",
				mcp.Description("Optional project description"),
			),
		),
		createProjectHandler(projects),
	)
}
```

### Handler Functions

```go
package mcpadapter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"myapp/internal/service"
)

// getUserHandler returns a tool handler that fetches a user by ID.
func getUserHandler(svc service.UserService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID, ok := request.Params.Arguments["user_id"].(string)
		if !ok || userID == "" {
			return mcp.NewToolResultError("user_id is required"), nil
		}

		user, err := svc.Get(ctx, userID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get user: %v", err)), nil
		}

		data, _ := json.MarshalIndent(user, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

// createUserHandler returns a tool handler that creates a new user.
func createUserHandler(svc service.UserService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, _ := request.Params.Arguments["name"].(string)
		email, _ := request.Params.Arguments["email"].(string)

		if name == "" || email == "" {
			return mcp.NewToolResultError("name and email are required"), nil
		}

		user, err := svc.Create(ctx, service.CreateUserInput{
			Name:  name,
			Email: email,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create user: %v", err)), nil
		}

		data, _ := json.MarshalIndent(user, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

// listUsersHandler returns a tool handler that lists all users.
func listUsersHandler(svc service.UserService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		users, err := svc.List(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list users: %v", err)), nil
		}

		if len(users) == 0 {
			return mcp.NewToolResultText("No users found."), nil
		}

		data, _ := json.MarshalIndent(users, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

// deleteUserHandler returns a tool handler that deletes a user by ID.
func deleteUserHandler(svc service.UserService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID, ok := request.Params.Arguments["user_id"].(string)
		if !ok || userID == "" {
			return mcp.NewToolResultError("user_id is required"), nil
		}

		if err := svc.Delete(ctx, userID); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to delete user: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("User %s deleted successfully.", userID)), nil
	}
}

// listProjectsHandler returns a tool handler that lists projects.
func listProjectsHandler(svc service.ProjectService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ownerID, _ := request.Params.Arguments["owner_id"].(string)

		projects, err := svc.List(ctx, service.ListProjectsInput{
			OwnerID: ownerID,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list projects: %v", err)), nil
		}

		data, _ := json.MarshalIndent(projects, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

// createProjectHandler returns a tool handler that creates a project.
func createProjectHandler(svc service.ProjectService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, _ := request.Params.Arguments["name"].(string)
		ownerID, _ := request.Params.Arguments["owner_id"].(string)
		description, _ := request.Params.Arguments["description"].(string)

		if name == "" || ownerID == "" {
			return mcp.NewToolResultError("name and owner_id are required"), nil
		}

		project, err := svc.Create(ctx, service.CreateProjectInput{
			Name:        name,
			OwnerID:     ownerID,
			Description: description,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create project: %v", err)), nil
		}

		data, _ := json.MarshalIndent(project, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}
```

### The MCP Adapter Struct

```go
package mcpadapter

import (
	"context"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"myapp/internal/adapter"
	"myapp/internal/service"
)

// Adapter is the MCP server adapter.
type Adapter struct {
	adapter.BaseAdapter
	mcpServer *server.MCPServer
	stdio     *server.StdioServer
}

// New creates a fully wired MCP adapter.
func New(
	logger *slog.Logger,
	users service.UserService,
	projects service.ProjectService,
) *Adapter {
	s := server.NewMCPServer(
		"myapp",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	registerTools(s, users, projects)

	return &Adapter{
		BaseAdapter: adapter.NewBaseAdapter("mcp", logger),
		mcpServer:   s,
		stdio:       server.NewStdioServer(s),
	}
}

// Start begins serving MCP over stdio. Blocks until ctx is cancelled.
func (a *Adapter) Start(ctx context.Context) error {
	a.SetHealth(true, "serving via stdio")
	a.Logger.Info("mcp adapter started (stdio)")

	if err := a.stdio.Listen(ctx, nil); err != nil {
		a.SetHealth(false, err.Error())
		return err
	}

	return nil
}

// Stop performs graceful shutdown.
func (a *Adapter) Stop(ctx context.Context) error {
	a.Logger.Info("mcp adapter stopping")
	a.SetHealth(false, "shutting down")
	// stdio server shuts down when context is cancelled
	return nil
}
```

### Wiring in main.go

```go
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	mcpadapter "myapp/internal/adapter/mcp"
	"myapp/internal/service"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Initialize services.
	userSvc := service.NewUserService( /* deps */ )
	projectSvc := service.NewProjectService( /* deps */ )

	// Create MCP adapter.
	adapter := mcpadapter.New(logger, userSvc, projectSvc)

	// Run with signal handling.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := adapter.Start(ctx); err != nil {
		logger.Error("mcp adapter error", "error", err)
		os.Exit(1)
	}
}
```

### MCP Server Configuration (claude_desktop_config.json)

```json
{
  "mcpServers": {
    "myapp": {
      "command": "/path/to/myapp",
      "args": ["mcp"],
      "env": {
        "DATABASE_URL": "postgres://localhost/myapp"
      }
    }
  }
}
```

## Benefits

1. **Service Reuse**: The same service layer that powers REST and CLI adapters is exposed to AI agents without duplication.
2. **Type-Safe Tool Definitions**: JSON Schema parameters on tools provide validation and documentation for AI consumers.
3. **Simple Handler Pattern**: Each handler is a closure over its service dependency, making it easy to add new tools.
4. **Stdio Simplicity**: The stdio transport requires no network configuration, making MCP servers easy to deploy and test.
5. **Consistent Lifecycle**: Implements the Base Adapter interface, so it integrates with the application's signal handling and health reporting.

## Best Practices

- Return `mcp.NewToolResultError()` for expected errors (not found, validation) rather than returning a Go error. A Go error from a handler signals a protocol-level failure.
- Format tool output as JSON for structured data and plain text for simple messages.
- Keep tool descriptions concise but specific. The AI agent reads these to decide which tool to call.
- Use `mcp.Required()` for mandatory parameters. Do not silently default required values.
- Log to stderr, not stdout. Stdout is the stdio transport channel.
- Group related tools by domain (users, projects) in the registration function for readability.

## Anti-Patterns

### Putting Business Logic in Handlers

**Bad**: Performing database queries or complex validation directly in tool handlers.

```go
// Bad: business logic in handler
func getUserHandler() server.ToolHandlerFunc {
    return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        userID := req.Params.Arguments["user_id"].(string)
        row := db.QueryRow("SELECT * FROM users WHERE id = $1", userID)
        // ...parsing, validation, transformation...
    }
}
```

**Good**: Call the service layer, which owns all business logic.

```go
// Good: thin adapter, service has the logic
func getUserHandler(svc service.UserService) server.ToolHandlerFunc {
    return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        userID := req.Params.Arguments["user_id"].(string)
        user, err := svc.Get(ctx, userID)
        // ...format and return...
    }
}
```

### Logging to Stdout

**Bad**: Using `fmt.Println` or a logger writing to stdout. This corrupts the stdio MCP transport.

**Good**: Always configure loggers to write to stderr.

### Monolithic Tool Registration

**Bad**: One giant function that defines 50 tools with inline handlers.

**Good**: Split tool definitions by domain and use handler factory functions.

## Related Patterns

- **[adapter-base](./core-sdk-go.adapter-base.md)**: The lifecycle interface this adapter implements.
- **[adapter-rest](./core-sdk-go.adapter-rest.md)**: HTTP adapter exposing the same services.
- **[adapter-cli](./core-sdk-go.adapter-cli.md)**: CLI adapter exposing the same services.

## Testing

### Unit Testing Tool Handlers

```go
package mcpadapter_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	mcpadapter "myapp/internal/adapter/mcp"
	"myapp/internal/service"
)

type mockUserService struct {
	getFn func(ctx context.Context, id string) (*service.User, error)
}

func (m *mockUserService) Get(ctx context.Context, id string) (*service.User, error) {
	return m.getFn(ctx, id)
}

// ... other methods

func TestGetUserHandler(t *testing.T) {
	mock := &mockUserService{
		getFn: func(_ context.Context, id string) (*service.User, error) {
			return &service.User{
				ID:    id,
				Name:  "Alice",
				Email: "alice@example.com",
			}, nil
		},
	}

	handler := mcpadapter.GetUserHandler(mock)

	result, err := handler(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"user_id": "usr_123",
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the result contains the expected user data.
	text := result.Content[0].(mcp.TextContent).Text
	var user service.User
	if err := json.Unmarshal([]byte(text), &user); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if user.Name != "Alice" {
		t.Fatalf("expected Alice, got %s", user.Name)
	}
}
```

### Testing Error Cases

```go
func TestGetUserHandler_NotFound(t *testing.T) {
	mock := &mockUserService{
		getFn: func(_ context.Context, id string) (*service.User, error) {
			return nil, service.ErrNotFound
		},
	}

	handler := mcpadapter.GetUserHandler(mock)

	result, err := handler(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"user_id": "nonexistent",
			},
		},
	})

	if err != nil {
		t.Fatalf("handler should not return Go error for expected failures")
	}

	if !result.IsError {
		t.Fatal("expected error result for not-found user")
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
