# Pattern: Config Loading

**Namespace**: core-sdk-go
**Category**: Configuration
**Created**: 2026-03-17
**Status**: Active

---

## Overview

The Config Loading pattern implements a layered configuration pipeline where multiple sources are merged in priority order: defaults, config files, environment variables, and CLI flags. This is the Go-idiomatic equivalent of the TypeScript core-sdk `loadConfig()` function that merges schema defaults, raw config objects, and `process.env` overrides. Go achieves this with libraries like koanf or viper that provide structured, composable config loading.

## Problem

Real-world applications need configuration from multiple sources with clear precedence rules. A developer's local YAML file should override defaults, environment variables in production should override the file, and CLI flags should override everything for debugging. The TypeScript core-sdk uses `deepMerge` with manual env mapping. Go needs a more structured approach that handles type coercion across all sources.

## Solution

Use koanf (lightweight, composable) or viper (batteries-included) to build a layered config pipeline. Each layer is loaded in priority order, with later layers overriding earlier ones. After merging, unmarshal into typed config structs and validate with go-playground/validator. Freeze the config by passing it as a value (not pointer) to consuming code.

## Implementation

### Approach 1: koanf (Recommended -- Lightweight)

```go
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

// Load builds AppConfig from layered sources.
// Priority (highest wins): CLI flags > env vars > config file > defaults.
func Load(configPath string, flags *pflag.FlagSet) (AppConfig, error) {
	k := koanf.New(".")

	// Layer 1: Defaults
	defaults := map[string]interface{}{
		"env":                       "development",
		"database.host":             "localhost",
		"database.port":             5432,
		"database.ssl":              false,
		"database.pool_min":         2,
		"database.pool_max":         10,
		"server.port":               3000,
		"server.host":               "0.0.0.0",
		"server.request_timeout_ms": 30000,
		"logging.level":             "info",
		"logging.format":            "json",
	}
	if err := k.Load(confmap.Provider(defaults, "."), nil); err != nil {
		return AppConfig{}, fmt.Errorf("loading defaults: %w", err)
	}

	// Layer 2: Config file (YAML)
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
				return AppConfig{}, fmt.Errorf("loading config file %s: %w", configPath, err)
			}
		}
	}

	// Layer 3: Environment variables
	// APP_DATABASE_HOST -> database.host
	if err := k.Load(env.Provider("APP_", ".", func(s string) string {
		return strings.Replace(
			strings.ToLower(strings.TrimPrefix(s, "APP_")),
			"_", ".", -1,
		)
	}), nil); err != nil {
		return AppConfig{}, fmt.Errorf("loading env vars: %w", err)
	}

	// Layer 4: CLI flags (only flags that were explicitly set)
	if flags != nil {
		if err := k.Load(posflag.Provider(flags, ".", k), nil); err != nil {
			return AppConfig{}, fmt.Errorf("loading CLI flags: %w", err)
		}
	}

	// Unmarshal into typed struct
	var cfg AppConfig
	if err := k.Unmarshal("", &cfg); err != nil {
		return AppConfig{}, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		return AppConfig{}, err
	}

	return cfg, nil
}
```

### Approach 2: viper (Batteries-Included)

```go
package config

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Load builds AppConfig using viper's layered config.
func Load(configPath string, flags *pflag.FlagSet) (AppConfig, error) {
	v := viper.New()

	// Layer 1: Defaults
	v.SetDefault("env", "development")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.ssl", false)
	v.SetDefault("database.pool_min", 2)
	v.SetDefault("database.pool_max", 10)
	v.SetDefault("server.port", 3000)
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.request_timeout_ms", 30000)
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")

	// Layer 2: Config file
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return AppConfig{}, fmt.Errorf("reading config file: %w", err)
		}
	}

	// Layer 3: Environment variables
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Layer 4: CLI flags
	if flags != nil {
		if err := v.BindPFlags(flags); err != nil {
			return AppConfig{}, fmt.Errorf("binding flags: %w", err)
		}
	}

	// Unmarshal into typed struct
	var cfg AppConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return AppConfig{}, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		return AppConfig{}, err
	}

	return cfg, nil
}
```

### Config File Example (config.yaml)

```yaml
env: production

database:
  host: db.prod.internal
  port: 5432
  name: myapp
  user: app_user
  password: "${DB_PASSWORD}"  # resolved by env layer
  ssl: true
  pool_min: 5
  pool_max: 25

server:
  port: 8080
  host: 0.0.0.0
  cors_origins:
    - https://app.example.com
  request_timeout_ms: 60000

logging:
  level: info
  format: json
```

### CLI Flag Integration with pflag

```go
package main

import (
	"fmt"
	"log"
	"os"

	"yourmodule/config"

	"github.com/spf13/pflag"
)

func main() {
	flags := pflag.NewFlagSet("app", pflag.ExitOnError)
	configPath := flags.String("config", "", "path to config file")
	flags.Int("server.port", 0, "server listen port")
	flags.String("logging.level", "", "log level (debug|info|warn|error)")
	flags.Parse(os.Args[1:])

	cfg, err := config.Load(*configPath, flags)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	fmt.Printf("Starting server on %s:%d (env=%s)\n",
		cfg.Server.Host, cfg.Server.Port, cfg.Env)
}
```

### Making Config Immutable After Loading

```go
// Pass config by value, not pointer. Receivers get a copy they cannot mutate.
func NewServer(cfg config.ServerConfig) *Server {
	return &Server{cfg: cfg} // cfg is a copy
}

// For extra safety, unexport the config field.
type Server struct {
	cfg config.ServerConfig // unexported -- cannot be modified externally
}

// Provide read-only accessors if needed.
func (s *Server) Port() int {
	return s.cfg.Port
}
```

### Complete Loading Pipeline

```
┌──────────────┐
│   Defaults   │  Hardcoded in code (lowest priority)
└──────┬───────┘
       ▼
┌──────────────┐
│  Config File │  YAML/JSON loaded from --config flag or default path
└──────┬───────┘
       ▼
┌──────────────┐
│  Env Vars    │  APP_DATABASE_HOST, APP_SERVER_PORT, etc.
└──────┬───────┘
       ▼
┌──────────────┐
│  CLI Flags   │  --server.port=9090 (highest priority)
└──────┬───────┘
       ▼
┌──────────────┐
│  Unmarshal   │  Into typed AppConfig struct
└──────┬───────┘
       ▼
┌──────────────┐
│  Validate    │  go-playground/validator checks constraints
└──────┬───────┘
       ▼
┌──────────────┐
│  Freeze      │  Pass by value to consumers
└──────────────┘
```

### TypeScript Comparison

| Concern              | TypeScript (core-sdk)              | Go (koanf/viper)                     |
|----------------------|------------------------------------|--------------------------------------|
| Default values       | Zod `.default()`                   | `SetDefault()` or `confmap`          |
| File loading         | Caller reads JSON/YAML             | Built-in file provider               |
| Env override         | Manual `process.env` mapping       | Automatic env provider with prefix   |
| CLI override         | Not built-in                       | pflag integration                    |
| Deep merge           | Custom `deepMerge()` function      | Built into koanf/viper               |
| Validation           | `Schema.parse(merged)`             | `validator.Struct()` after unmarshal |

## Benefits

1. **Clear precedence** -- each layer has a well-defined priority, eliminating ambiguity about which value wins.
2. **Separation of concerns** -- defaults, files, env, and flags are loaded independently and merged automatically.
3. **Type-safe output** -- the final result is a validated, typed struct, not a loosely-typed map.
4. **File format flexibility** -- koanf/viper support YAML, JSON, TOML, and more with the same API.
5. **Testable** -- each layer can be tested independently; the full pipeline can be tested with controlled inputs.

## Best Practices

- Call `Load()` once in `main()` and pass the resulting config struct to all constructors.
- Use the `APP_` prefix for all env vars to avoid collisions with system variables.
- Only override defaults via CLI flags for values operators need to change at runtime (e.g., port, log level).
- Log the loaded config at startup (with secrets redacted) for operational visibility.
- Fail fast: if `Load()` returns an error, terminate immediately with a clear message.
- Keep the config file optional: the app should run with just env vars or defaults.

## Anti-Patterns

### Loading config lazily or repeatedly

Config should be loaded once at startup. Lazy loading introduces race conditions and makes it hard to reason about which values are active.

### Using viper's global instance

Viper's `viper.Get()` global functions create hidden state. Always use `viper.New()` for an isolated instance, or prefer koanf which has no global state.

```go
// BAD: global viper
func GetPort() int {
    return viper.GetInt("server.port")
}

// GOOD: explicit config
func NewServer(cfg ServerConfig) *Server {
    return &Server{port: cfg.Port}
}
```

### Skipping validation after unmarshal

Unmarshaling does not validate constraints. Always call `Validate()` after `Unmarshal()`.

## Related Patterns

- **[config-struct](./core-sdk-go.config-struct.md)** -- the struct definitions and validation rules that config is loaded into.
- **[config-environment](./core-sdk-go.config-environment.md)** -- standalone env var loading when the full pipeline is not needed.
- **[config-secrets](./core-sdk-go.config-secrets.md)** -- wrapping sensitive fields so they are redacted in logs and serialization.

## Testing

```go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"yourmodule/config"

	"github.com/spf13/pflag"
)

func TestLoad_DefaultsOnly(t *testing.T) {
	// Provide only required fields via env
	t.Setenv("APP_DATABASE_NAME", "testdb")
	t.Setenv("APP_DATABASE_USER", "testuser")
	t.Setenv("APP_DATABASE_PASSWORD", "testpass")

	cfg, err := config.Load("", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 3000 {
		t.Errorf("expected default server port 3000, got %d", cfg.Server.Port)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected default log level 'info', got %q", cfg.Logging.Level)
	}
}

func TestLoad_FileOverridesDefaults(t *testing.T) {
	configYAML := `
server:
  port: 9090
logging:
  level: debug
database:
  name: filedb
  user: fileuser
  password: filepass
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configPath, []byte(configYAML), 0644)

	cfg, err := config.Load(configPath, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090 from file, got %d", cfg.Server.Port)
	}
	if string(cfg.Logging.Level) != "debug" {
		t.Errorf("expected level 'debug' from file, got %q", cfg.Logging.Level)
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	configYAML := `
server:
  port: 9090
database:
  name: filedb
  user: fileuser
  password: filepass
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configPath, []byte(configYAML), 0644)

	t.Setenv("APP_SERVER_PORT", "4000")

	cfg, err := config.Load(configPath, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 4000 {
		t.Errorf("expected env override port 4000, got %d", cfg.Server.Port)
	}
}

func TestLoad_FlagsOverrideEnv(t *testing.T) {
	t.Setenv("APP_DATABASE_NAME", "testdb")
	t.Setenv("APP_DATABASE_USER", "testuser")
	t.Setenv("APP_DATABASE_PASSWORD", "testpass")
	t.Setenv("APP_SERVER_PORT", "4000")

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.Int("server.port", 0, "server port")
	flags.Parse([]string{"--server.port=5555"})

	cfg, err := config.Load("", flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 5555 {
		t.Errorf("expected flag override port 5555, got %d", cfg.Server.Port)
	}
}

func TestLoad_ValidationFailure(t *testing.T) {
	// Missing required database fields
	_, err := config.Load("", nil)
	if err == nil {
		t.Fatal("expected validation error for missing required fields")
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
