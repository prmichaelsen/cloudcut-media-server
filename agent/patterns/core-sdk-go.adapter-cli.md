# Pattern: CLI Adapter

**Namespace**: core-sdk-go
**Category**: Adapter Layer
**Created**: 2026-03-17
**Status**: Active

---

## Overview

The CLI Adapter pattern implements a command-line interface using `cobra` that translates CLI invocations into service-layer calls. It maps flags and arguments to service inputs, formats service outputs for terminal display, and translates service errors into appropriate exit codes and user-friendly messages. Like all adapters, the CLI is a thin transport layer with no business logic.

## Problem

CLI tools in Go often embed business logic directly in cobra command `RunE` functions, making the same logic unavailable to other adapters (REST, MCP). Error handling becomes inconsistent: some commands print errors, others use `log.Fatal`, and exit codes are arbitrary. Output formatting is scattered across commands instead of being centralized.

## Solution

Create a CLI adapter struct that owns the root cobra command and wires services into subcommands. Each command's `RunE` function parses flags, calls a service method, and passes the result to a formatter. Error handling is centralized: service errors are mapped to exit codes and user-friendly messages. Output formatting supports multiple modes (table, JSON, plain text) via a flag.

## Implementation

### Project Structure

```
internal/
  adapter/
    cli/
      cli.go           # Adapter struct, root command, Start/Stop
      users.go          # User subcommands
      projects.go       # Project subcommands
      format.go         # Output formatting (table, JSON, text)
      errors.go         # Error-to-exit-code mapping
  service/
    user.go
    project.go
```

### Exit Codes and Error Mapping

```go
package cli

import (
	"errors"
	"fmt"
	"os"

	"myapp/internal/service"
)

const (
	ExitOK             = 0
	ExitGeneralError   = 1
	ExitUsageError     = 2
	ExitNotFound       = 3
	ExitConflict       = 4
	ExitUnauthorized   = 5
	ExitValidation     = 6
)

// mapExitCode maps a service error to a CLI exit code.
func mapExitCode(err error) int {
	switch {
	case errors.Is(err, service.ErrNotFound):
		return ExitNotFound
	case errors.Is(err, service.ErrConflict):
		return ExitConflict
	case errors.Is(err, service.ErrUnauthorized):
		return ExitUnauthorized
	case errors.Is(err, service.ErrValidation):
		return ExitValidation
	default:
		return ExitGeneralError
	}
}

// userFriendlyError returns a message suitable for terminal display.
func userFriendlyError(err error) string {
	switch {
	case errors.Is(err, service.ErrNotFound):
		return "Error: resource not found"
	case errors.Is(err, service.ErrConflict):
		return "Error: resource already exists"
	case errors.Is(err, service.ErrValidation):
		return fmt.Sprintf("Error: invalid input: %v", err)
	case errors.Is(err, service.ErrUnauthorized):
		return "Error: not authorized. Check your credentials."
	default:
		return fmt.Sprintf("Error: %v", err)
	}
}

// handleError prints a user-friendly message to stderr and exits.
func handleError(err error) {
	fmt.Fprintln(os.Stderr, userFriendlyError(err))
	os.Exit(mapExitCode(err))
}
```

### Output Formatting

```go
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// OutputFormat controls how results are printed.
type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
	FormatPlain OutputFormat = "plain"
)

// Printer handles output formatting.
type Printer struct {
	Out    io.Writer
	Format OutputFormat
}

// PrintTable writes tabular data with aligned columns.
func (p *Printer) PrintTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(p.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	fmt.Fprintln(w, strings.Repeat("-\t", len(headers)))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	w.Flush()
}

// PrintJSON writes a value as indented JSON.
func (p *Printer) PrintJSON(v any) error {
	enc := json.NewEncoder(p.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// PrintPlain writes a simple string.
func (p *Printer) PrintPlain(msg string) {
	fmt.Fprintln(p.Out, msg)
}

// Print outputs data in the configured format.
// tableData should provide headers and rows; v is used for JSON.
func (p *Printer) Print(v any, headers []string, rows [][]string) {
	switch p.Format {
	case FormatJSON:
		p.PrintJSON(v)
	case FormatPlain:
		for _, row := range rows {
			fmt.Fprintln(p.Out, strings.Join(row, " "))
		}
	default:
		p.PrintTable(headers, rows)
	}
}
```

### User Subcommands

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"myapp/internal/service"
)

func newUsersCmd(users service.UserService, printer *Printer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Manage users",
	}

	cmd.AddCommand(
		newUsersListCmd(users, printer),
		newUsersGetCmd(users, printer),
		newUsersCreateCmd(users, printer),
		newUsersDeleteCmd(users, printer),
	)

	return cmd
}

func newUsersListCmd(users service.UserService, printer *Printer) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all users",
		RunE: func(cmd *cobra.Command, args []string) error {
			list, err := users.List(cmd.Context())
			if err != nil {
				return err
			}

			headers := []string{"ID", "NAME", "EMAIL"}
			rows := make([][]string, len(list))
			for i, u := range list {
				rows[i] = []string{u.ID, u.Name, u.Email}
			}

			printer.Print(list, headers, rows)
			return nil
		},
	}
}

func newUsersGetCmd(users service.UserService, printer *Printer) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a user by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			user, err := users.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			headers := []string{"ID", "NAME", "EMAIL"}
			rows := [][]string{{user.ID, user.Name, user.Email}}

			printer.Print(user, headers, rows)
			return nil
		},
	}
}

func newUsersCreateCmd(users service.UserService, printer *Printer) *cobra.Command {
	var name, email string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new user",
		RunE: func(cmd *cobra.Command, args []string) error {
			user, err := users.Create(cmd.Context(), service.CreateUserInput{
				Name:  name,
				Email: email,
			})
			if err != nil {
				return err
			}

			printer.Print(user,
				[]string{"ID", "NAME", "EMAIL"},
				[][]string{{user.ID, user.Name, user.Email}},
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "User name (required)")
	cmd.Flags().StringVar(&email, "email", "", "User email (required)")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("email")

	return cmd
}

func newUsersDeleteCmd(users service.UserService, printer *Printer) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a user by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := users.Delete(cmd.Context(), args[0]); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "User %s deleted.\n", args[0])
			return nil
		},
	}
}
```

### The CLI Adapter Struct

```go
package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"myapp/internal/adapter"
	"myapp/internal/service"
)

// Adapter is the CLI adapter.
type Adapter struct {
	adapter.BaseAdapter
	root *cobra.Command
}

// New creates a fully wired CLI adapter.
func New(
	logger *slog.Logger,
	users service.UserService,
	projects service.ProjectService,
) *Adapter {
	a := &Adapter{
		BaseAdapter: adapter.NewBaseAdapter("cli", logger),
	}

	// Output format flag, shared across all commands.
	var outputFormat string

	root := &cobra.Command{
		Use:   "myapp",
		Short: "MyApp CLI",
		Long:  "MyApp command-line interface for managing users and projects.",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Make the printer available to all subcommands.
			cmd.SetContext(context.WithValue(cmd.Context(), "printer", &Printer{
				Out:    cmd.OutOrStdout(),
				Format: OutputFormat(outputFormat),
			}))
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table",
		"Output format: table, json, plain")

	// Create printer (default, overridden in PersistentPreRun).
	printer := &Printer{Out: os.Stdout, Format: FormatTable}

	// Register subcommands.
	root.AddCommand(
		newUsersCmd(users, printer),
		newProjectsCmd(projects, printer),
		newVersionCmd(),
	)

	a.root = root
	return a
}

// Start executes the CLI command tree. Blocks until the command completes.
func (a *Adapter) Start(ctx context.Context) error {
	a.SetHealth(true, "running")
	a.root.SetContext(ctx)

	if err := a.root.Execute(); err != nil {
		a.SetHealth(false, err.Error())
		fmt.Fprintln(os.Stderr, userFriendlyError(err))
		os.Exit(mapExitCode(err))
	}

	a.SetHealth(false, "completed")
	return nil
}

// Stop is a no-op for CLI (runs to completion).
func (a *Adapter) Stop(_ context.Context) error {
	return nil
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the application version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), "myapp v1.0.0")
		},
	}
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

	cliadapter "myapp/internal/adapter/cli"
	"myapp/internal/service"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	userSvc := service.NewUserService( /* deps */ )
	projectSvc := service.NewProjectService( /* deps */ )

	adapter := cliadapter.New(logger, userSvc, projectSvc)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := adapter.Start(ctx); err != nil {
		os.Exit(1)
	}
}
```

### Example CLI Session

```bash
# List users as a table (default)
$ myapp users list
ID          NAME    EMAIL
--          ----    -----
usr_001     Alice   alice@example.com
usr_002     Bob     bob@example.com

# Get a single user as JSON
$ myapp users get usr_001 -o json
{
  "id": "usr_001",
  "name": "Alice",
  "email": "alice@example.com"
}

# Create a user
$ myapp users create --name "Charlie" --email "charlie@example.com"
ID          NAME      EMAIL
--          ----      -----
usr_003     Charlie   charlie@example.com

# Delete a user
$ myapp users delete usr_003
User usr_003 deleted.

# Error case
$ myapp users get nonexistent
Error: resource not found
$ echo $?
3
```

## Benefits

1. **Service Reuse**: The same service layer powers CLI, REST, and MCP with no duplication.
2. **Consistent Error Handling**: All commands map service errors to the same exit codes and messages.
3. **Flexible Output**: Users choose table, JSON, or plain text via `--output`, making the CLI scriptable.
4. **Discoverable**: Cobra provides built-in help, autocompletion, and usage documentation.
5. **Testable**: Commands can be tested by executing the cobra command with captured stdout/stderr.

## Best Practices

- Use `RunE` (not `Run`) so errors propagate to the root command's error handler.
- Set `SilenceUsage: true` and `SilenceErrors: true` on the root command; handle errors yourself for consistent formatting.
- Use `cobra.ExactArgs(n)` for positional arguments. Use flags for optional or named parameters.
- Pass `cmd.Context()` to service calls so that signal cancellation propagates.
- Keep `--output` as a persistent flag on the root command so all subcommands inherit it.
- Use `cmd.OutOrStdout()` and `cmd.ErrOrStderr()` so tests can capture output.

## Anti-Patterns

### Business Logic in RunE

**Bad**: Performing database queries or complex validation inside a cobra command.

```go
// Bad
RunE: func(cmd *cobra.Command, args []string) error {
    db := sql.Open("postgres", os.Getenv("DATABASE_URL"))
    row := db.QueryRow("SELECT * FROM users WHERE id = $1", args[0])
    // ... parsing, validation ...
}
```

**Good**: Call the injected service.

```go
// Good
RunE: func(cmd *cobra.Command, args []string) error {
    user, err := userSvc.Get(cmd.Context(), args[0])
    // ... format and print ...
}
```

### Using log.Fatal for Errors

**Bad**: `log.Fatal` exits immediately without cleanup and does not set meaningful exit codes.

**Good**: Return errors from `RunE` and handle them in the root command's error path with proper exit codes.

### Hardcoding Output Format

**Bad**: Always printing tables, making the CLI unusable in scripts that need JSON.

**Good**: Support `--output json` for machine-readable output.

## Related Patterns

- **[adapter-base](./core-sdk-go.adapter-base.md)**: The lifecycle interface this adapter implements.
- **[adapter-rest](./core-sdk-go.adapter-rest.md)**: HTTP adapter exposing the same services.
- **[adapter-mcp](./core-sdk-go.adapter-mcp.md)**: MCP adapter exposing the same services.

## Testing

### Testing Commands with Captured Output

```go
package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	cliadapter "myapp/internal/adapter/cli"
	"myapp/internal/service"
)

func TestUsersListCmd(t *testing.T) {
	mock := &mockUserService{
		listFn: func(_ context.Context) ([]*service.User, error) {
			return []*service.User{
				{ID: "usr_1", Name: "Alice", Email: "alice@example.com"},
			}, nil
		},
	}

	adapter := cliadapter.New(nil, mock, nil)
	buf := new(bytes.Buffer)
	adapter.Root().SetOut(buf)
	adapter.Root().SetArgs([]string{"users", "list"})

	if err := adapter.Root().Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Alice") {
		t.Fatalf("expected output to contain Alice, got: %s", output)
	}
}
```

### Testing JSON Output

```go
func TestUsersGetCmd_JSON(t *testing.T) {
	mock := &mockUserService{
		getFn: func(_ context.Context, id string) (*service.User, error) {
			return &service.User{ID: id, Name: "Bob", Email: "bob@example.com"}, nil
		},
	}

	adapter := cliadapter.New(nil, mock, nil)
	buf := new(bytes.Buffer)
	adapter.Root().SetOut(buf)
	adapter.Root().SetArgs([]string{"users", "get", "usr_1", "-o", "json"})

	if err := adapter.Root().Execute(); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), `"name": "Bob"`) {
		t.Fatalf("expected JSON output with Bob, got: %s", buf.String())
	}
}
```

### Testing Error Exit Codes

```go
func TestUsersGetCmd_NotFound(t *testing.T) {
	mock := &mockUserService{
		getFn: func(_ context.Context, id string) (*service.User, error) {
			return nil, service.ErrNotFound
		},
	}

	adapter := cliadapter.New(nil, mock, nil)
	adapter.Root().SetArgs([]string{"users", "get", "nonexistent"})

	err := adapter.Root().Execute()
	if err == nil {
		t.Fatal("expected error for not-found user")
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
