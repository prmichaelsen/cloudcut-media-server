# Pattern: Config Environment

**Namespace**: core-sdk-go
**Category**: Configuration
**Created**: 2026-03-17
**Status**: Active

---

## Overview

The Config Environment pattern handles loading configuration values from environment variables into Go structs. This is the Go-idiomatic equivalent of the TypeScript core-sdk pattern where `process.env` values are manually mapped and type-coerced in the config loader. Go offers both a manual approach with `os.Getenv()` and a tag-based approach using libraries like `envconfig` or `env`.

## Problem

Environment variables are always strings. Applications need to parse them into typed Go values (int, bool, duration, slices), handle missing-vs-empty distinctions, apply defaults, enforce required fields, and support namespaced prefixes to avoid collisions between services. Doing this manually for every field is error-prone and repetitive.

## Solution

Use the `kelseyhightower/envconfig` library (or `caarlos0/env`) with struct tags to declaratively map environment variables to config struct fields. The library handles type coercion, defaults, required enforcement, and prefix namespacing automatically. For simple cases or minimal dependencies, use a manual `os.Getenv()` + `strconv` approach.

## Implementation

### Approach 1: Manual with os.Getenv (Zero Dependencies)

```go
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// LoadDatabaseFromEnv populates DatabaseConfig from environment variables.
func LoadDatabaseFromEnv() (DatabaseConfig, error) {
	cfg := DatabaseConfig{
		Host:    "localhost",
		Port:    5432,
		SSL:     false,
		PoolMin: 2,
		PoolMax: 10,
	}

	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("DB_PORT: %w", err)
		}
		cfg.Port = port
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.Name = v
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.User = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.Password = v
	}
	if v := os.Getenv("DB_SSL"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return cfg, fmt.Errorf("DB_SSL: %w", err)
		}
		cfg.SSL = b
	}

	return cfg, nil
}

// Helper for parsing durations from env.
func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}

// Helper for parsing comma-separated string slices from env.
func envStringSlice(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
```

### Approach 2: Tag-Based with envconfig (Recommended)

```go
package config

import (
	"github.com/kelseyhightower/envconfig"
)

// DatabaseConfig uses envconfig struct tags.
// The prefix is set when calling envconfig.Process.
type DatabaseConfig struct {
	Host     string `envconfig:"HOST"     default:"localhost"`
	Port     int    `envconfig:"PORT"     default:"5432"`
	Name     string `envconfig:"NAME"     required:"true"`
	User     string `envconfig:"USER"     required:"true"`
	Password string `envconfig:"PASSWORD" required:"true"`
	SSL      bool   `envconfig:"SSL"      default:"false"`
	PoolMin  int    `envconfig:"POOL_MIN" default:"2"`
	PoolMax  int    `envconfig:"POOL_MAX" default:"10"`
}

type ServerConfig struct {
	Port             int    `envconfig:"PORT"               default:"3000"`
	Host             string `envconfig:"HOST"               default:"0.0.0.0"`
	CORSOrigins      []string `envconfig:"CORS_ORIGINS"`
	RequestTimeoutMs int    `envconfig:"REQUEST_TIMEOUT_MS" default:"30000"`
}

type LoggingConfig struct {
	Level  string `envconfig:"LEVEL"  default:"info"`
	Format string `envconfig:"FORMAT" default:"json"`
}

// AppConfig composes all sub-configs.
type AppConfig struct {
	Env      string         `envconfig:"APP_ENV" default:"development"`
	Database DatabaseConfig
	Server   ServerConfig
	Logging  LoggingConfig
}

// LoadFromEnv loads the entire AppConfig from environment variables.
// Each sub-config gets a prefix: DB_, SERVER_, LOG_.
func LoadFromEnv() (AppConfig, error) {
	var cfg AppConfig

	// Top-level fields (no prefix)
	if err := envconfig.Process("", &cfg); err != nil {
		return cfg, err
	}

	// Sub-configs with prefixes
	if err := envconfig.Process("DB", &cfg.Database); err != nil {
		return cfg, err
	}
	if err := envconfig.Process("SERVER", &cfg.Server); err != nil {
		return cfg, err
	}
	if err := envconfig.Process("LOG", &cfg.Logging); err != nil {
		return cfg, err
	}

	return cfg, nil
}
```

### Prefix Handling for Namespaced Variables

```go
// With envconfig, prefix "DB" maps:
//   DB_HOST     -> DatabaseConfig.Host
//   DB_PORT     -> DatabaseConfig.Port
//   DB_NAME     -> DatabaseConfig.Name
//
// With prefix "SERVER":
//   SERVER_PORT -> ServerConfig.Port
//   SERVER_HOST -> ServerConfig.Host
//
// This prevents collisions when multiple services run in the same
// environment and share variable namespaces.
```

### Approach 3: caarlos0/env (Alternative)

```go
package config

import (
	"github.com/caarlos0/env/v10"
)

// AppConfig using caarlos0/env tags.
type AppConfig struct {
	Env string `env:"APP_ENV" envDefault:"development"`

	DBHost     string `env:"DB_HOST"     envDefault:"localhost"`
	DBPort     int    `env:"DB_PORT"     envDefault:"5432"`
	DBName     string `env:"DB_NAME,required"`
	DBUser     string `env:"DB_USER,required"`
	DBPassword string `env:"DB_PASSWORD,required"`

	ServerPort int    `env:"PORT"        envDefault:"3000"`
	ServerHost string `env:"SERVER_HOST" envDefault:"0.0.0.0"`

	LogLevel  string `env:"LOG_LEVEL"  envDefault:"info"`
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`
}

func LoadFromEnv() (AppConfig, error) {
	var cfg AppConfig
	if err := env.Parse(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
```

### Type Coercion Reference

envconfig and caarlos0/env handle these conversions automatically:

| Env Value          | Go Type          | Result                          |
|--------------------|------------------|---------------------------------|
| `"3000"`           | `int`            | `3000`                          |
| `"true"`           | `bool`           | `true`                          |
| `"5s"`             | `time.Duration`  | `5 * time.Second`               |
| `"a,b,c"`          | `[]string`       | `["a", "b", "c"]`              |
| `"http://a http://b"` | `[]string`   | `["http://a", "http://b"]` (space-delimited in envconfig) |
| `"3.14"`           | `float64`        | `3.14`                          |

### TypeScript Comparison

In the TypeScript core-sdk, env loading is done manually in `loader.ts`:

```typescript
// TypeScript approach -- manual mapping with parseInt
const merged = deepMerge(raw, {
  database: {
    host: env.DB_HOST,
    port: env.DB_PORT ? parseInt(env.DB_PORT) : undefined,
  },
});
```

The Go envconfig approach replaces this with declarative struct tags, eliminating manual `parseInt`/`strconv` calls and `undefined` checks.

## Benefits

1. **Declarative mapping** -- struct tags document which env var maps to which field, serving as living documentation.
2. **Automatic type coercion** -- no manual `strconv.Atoi` or `strconv.ParseBool` calls.
3. **Required field enforcement** -- the library returns clear errors for missing required variables.
4. **Prefix namespacing** -- prevents variable name collisions across sub-configs.
5. **Default values** -- specified inline with the struct definition.

## Best Practices

- Use consistent prefix naming: `DB_`, `SERVER_`, `LOG_`, `APP_`.
- Prefer `envconfig` or `caarlos0/env` over manual `os.Getenv` for anything beyond 3-4 variables.
- Always validate the loaded config with go-playground/validator after env loading (see config-struct pattern).
- Document expected environment variables in a `.env.example` file at the project root.
- Use the manual approach only for simple CLIs or when you need zero external dependencies.
- For slices, document the delimiter (comma vs space) since libraries differ.

## Anti-Patterns

### Calling os.Getenv deep in application code

Environment variables should be read once at startup and mapped into the config struct. Never call `os.Getenv` from service or handler code.

```go
// BAD: reading env in handler
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    dbHost := os.Getenv("DB_HOST") // Don't do this
}

// GOOD: config was loaded at startup and injected
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    dbHost := h.cfg.Database.Host
}
```

### Ignoring errors from strconv

Always handle parse errors. A typo in an env var (e.g., `DB_PORT=abc`) should fail fast at startup, not cause a silent zero value.

### Mixing env var names and struct tag names inconsistently

If using envconfig with prefix "DB", the env var is `DB_HOST`, not `DATABASE_HOST`. Pick a convention and stick with it.

## Related Patterns

- **[config-struct](./core-sdk-go.config-struct.md)** -- defines the struct types that env vars are loaded into.
- **[config-loading](./core-sdk-go.config-loading.md)** -- orchestrates env loading as one layer in the full config pipeline.
- **[config-secrets](./core-sdk-go.config-secrets.md)** -- `Secret` type for sensitive env vars like passwords and API keys.

## Testing

```go
package config_test

import (
	"os"
	"testing"

	"yourmodule/config"
)

func TestLoadFromEnv_Defaults(t *testing.T) {
	// Set only required fields
	t.Setenv("DB_NAME", "testdb")
	t.Setenv("DB_USER", "testuser")
	t.Setenv("DB_PASSWORD", "testpass")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Database.Host != "localhost" {
		t.Errorf("expected default host 'localhost', got %q", cfg.Database.Host)
	}
	if cfg.Database.Port != 5432 {
		t.Errorf("expected default port 5432, got %d", cfg.Database.Port)
	}
}

func TestLoadFromEnv_Overrides(t *testing.T) {
	t.Setenv("DB_NAME", "mydb")
	t.Setenv("DB_USER", "admin")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_HOST", "db.example.com")
	t.Setenv("DB_PORT", "5433")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Database.Host != "db.example.com" {
		t.Errorf("expected host 'db.example.com', got %q", cfg.Database.Host)
	}
	if cfg.Database.Port != 5433 {
		t.Errorf("expected port 5433, got %d", cfg.Database.Port)
	}
}

func TestLoadFromEnv_MissingRequired(t *testing.T) {
	// Clear all DB env vars
	os.Unsetenv("DB_NAME")
	os.Unsetenv("DB_USER")
	os.Unsetenv("DB_PASSWORD")

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected error for missing required fields")
	}
}

func TestLoadFromEnv_InvalidPort(t *testing.T) {
	t.Setenv("DB_NAME", "testdb")
	t.Setenv("DB_USER", "testuser")
	t.Setenv("DB_PASSWORD", "testpass")
	t.Setenv("DB_PORT", "not-a-number")

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected error for non-numeric port")
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
