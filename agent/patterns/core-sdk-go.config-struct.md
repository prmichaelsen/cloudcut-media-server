# Pattern: Config Struct

**Namespace**: core-sdk-go
**Category**: Configuration
**Created**: 2026-03-17
**Status**: Active

---

## Overview

The Config Struct pattern defines application configuration as strongly-typed Go structs with struct tags for validation, environment mapping, and serialization. This is the Go-idiomatic equivalent of the TypeScript core-sdk patterns `config-schema` (Zod schemas) and `types-config` (derived types). In Go, the struct definition serves as both the schema and the type -- there is no separate schema-to-type derivation step.

## Problem

Applications need a single source of truth for configuration shape, defaults, validation rules, and serialization behavior. In TypeScript, Zod schemas define the shape and `z.infer` derives the types. Go lacks runtime schema-to-type inference, so a different approach is needed that still provides validation, defaults, and type safety without code duplication.

## Solution

Define configuration as plain Go structs annotated with struct tags:

- `validate` tags for runtime validation via go-playground/validator
- `json` and `yaml` tags for file deserialization
- `env` tags for environment variable mapping (used by envconfig)
- Zero values serve as implicit defaults; explicit defaults use the `default` tag

Use go-playground/validator for runtime validation after loading, replacing Zod's parse-time validation.

## Implementation

### Directory Structure

```
config/
    config.go       // Struct definitions + validation
    config_test.go  // Validation and default tests
```

### Core Config Structs

```go
package config

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	Host    string `json:"host"    yaml:"host"    env:"DB_HOST"     validate:"required"          default:"localhost"`
	Port    int    `json:"port"    yaml:"port"    env:"DB_PORT"     validate:"required,min=1,max=65535" default:"5432"`
	Name    string `json:"name"    yaml:"name"    env:"DB_NAME"     validate:"required"`
	User    string `json:"user"    yaml:"user"    env:"DB_USER"     validate:"required"`
	Password string `json:"password" yaml:"password" env:"DB_PASSWORD" validate:"required"`
	SSL     bool   `json:"ssl"     yaml:"ssl"     env:"DB_SSL"      default:"false"`
	PoolMin int    `json:"poolMin" yaml:"pool_min" env:"DB_POOL_MIN" validate:"min=0"             default:"2"`
	PoolMax int    `json:"poolMax" yaml:"pool_max" env:"DB_POOL_MAX" validate:"min=1,gtefield=PoolMin" default:"10"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port             int      `json:"port"             yaml:"port"              env:"PORT"                validate:"required,min=1,max=65535" default:"3000"`
	Host             string   `json:"host"             yaml:"host"              env:"SERVER_HOST"         validate:"required"                 default:"0.0.0.0"`
	CORSOrigins      []string `json:"corsOrigins"      yaml:"cors_origins"      env:"CORS_ORIGINS"`
	RequestTimeoutMs int      `json:"requestTimeoutMs" yaml:"request_timeout_ms" env:"REQUEST_TIMEOUT_MS" validate:"min=0"                    default:"30000"`
}

// LogLevel constrains allowed log levels.
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LogFormat constrains allowed log formats.
type LogFormat string

const (
	LogFormatJSON   LogFormat = "json"
	LogFormatPretty LogFormat = "pretty"
)

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  LogLevel  `json:"level"  yaml:"level"  env:"LOG_LEVEL"  validate:"required,oneof=debug info warn error" default:"info"`
	Format LogFormat `json:"format" yaml:"format" env:"LOG_FORMAT" validate:"required,oneof=json pretty"           default:"json"`
}

// Environment constrains allowed deployment environments.
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvStaging     Environment = "staging"
	EnvProduction  Environment = "production"
)

// AppConfig is the top-level application configuration.
// It composes sub-configs via embedding for flat access where convenient.
type AppConfig struct {
	Env      Environment    `json:"env"      yaml:"env"      env:"APP_ENV" validate:"required,oneof=development staging production" default:"development"`
	Database DatabaseConfig `json:"database" yaml:"database" validate:"required"`
	Server   ServerConfig   `json:"server"   yaml:"server"   validate:"required"`
	Logging  LoggingConfig  `json:"logging"  yaml:"logging"  validate:"required"`
}

// Validate runs struct validation on the entire config tree.
func (c *AppConfig) Validate() error {
	v := validator.New()
	if err := v.Struct(c); err != nil {
		var msgs []string
		for _, fe := range err.(validator.ValidationErrors) {
			msgs = append(msgs, fmt.Sprintf("field %s failed on '%s' (value: %v)", fe.Namespace(), fe.Tag(), fe.Value()))
		}
		return fmt.Errorf("config validation failed:\n  %s", strings.Join(msgs, "\n  "))
	}
	return nil
}
```

### Layer-Scoped Config Slices

```go
// ServiceConfig provides only the config sections needed by the service layer.
type ServiceConfig struct {
	Database DatabaseConfig
	Logging  LoggingConfig
}

// AdapterConfig provides only the config sections needed by the adapter layer.
type AdapterConfig struct {
	Server  ServerConfig
	Logging LoggingConfig
}

// ServiceCfg returns a ServiceConfig slice from the full AppConfig.
func (c *AppConfig) ServiceCfg() ServiceConfig {
	return ServiceConfig{
		Database: c.Database,
		Logging:  c.Logging,
	}
}

// AdapterCfg returns an AdapterConfig slice from the full AppConfig.
func (c *AppConfig) AdapterCfg() AdapterConfig {
	return AdapterConfig{
		Server:  c.Server,
		Logging: c.Logging,
	}
}
```

### Zero Values as Defaults

Go zero values provide implicit defaults for many types:

| Type     | Zero Value | Behavior                          |
|----------|-----------|-----------------------------------|
| `string` | `""`      | Empty string; use `default` tag for non-empty |
| `int`    | `0`       | Often needs explicit default via tag |
| `bool`   | `false`   | False by default; works naturally for opt-in flags |
| `[]T`    | `nil`     | Empty slice; use `len()` checks   |

For explicit defaults beyond zero values, use the `default` struct tag processed during config loading (see config-loading pattern).

### TypeScript Zod Comparison

| Concern         | TypeScript (Zod)                      | Go (Struct Tags)                       |
|-----------------|---------------------------------------|----------------------------------------|
| Schema definition | `z.object({...})`                   | `type Config struct{...}`              |
| Type derivation   | `z.infer<typeof Schema>`            | N/A -- struct IS the type              |
| Validation        | `.parse()` at load time             | `validator.Struct()` after load        |
| Defaults          | `.default("value")`                 | `default:"value"` tag or zero values   |
| Enums             | `z.enum(["a","b"])`                 | `const` + `oneof=a b` tag             |
| Nested schemas    | `z.object({sub: SubSchema})`        | Nested/embedded structs                |

### Test Helper

```go
package config

// CreateTestConfig returns a valid AppConfig with sensible test defaults.
// Override specific fields after calling.
func CreateTestConfig() AppConfig {
	return AppConfig{
		Env: EnvDevelopment,
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			Name:     "test_db",
			User:     "test",
			Password: "test",
			PoolMin:  2,
			PoolMax:  10,
		},
		Server: ServerConfig{
			Port:             3000,
			Host:             "0.0.0.0",
			RequestTimeoutMs: 30000,
		},
		Logging: LoggingConfig{
			Level:  LogLevelError,
			Format: LogFormatPretty,
		},
	}
}
```

## Benefits

1. **Single source of truth** -- the struct is the schema, the type, and the documentation all at once.
2. **Compile-time type safety** -- Go's type system catches field name and type mismatches at build time.
3. **Rich validation** -- go-playground/validator supports complex rules including cross-field checks (`gtefield`), ranges, and custom validators.
4. **IDE support** -- struct field autocompletion and type hints work out of the box.
5. **No code generation** -- unlike protobuf or Zod's `infer`, no build step is needed.

## Best Practices

- Define each config section as its own struct; compose via nesting in `AppConfig`.
- Always call `Validate()` immediately after loading config, before passing to any service.
- Export `CreateTestConfig()` so tests across packages share consistent defaults.
- Use typed constants (e.g., `LogLevel`, `Environment`) instead of raw strings for enum-like fields.
- Use `validate:"required"` for fields that must be explicitly set by the operator.
- Keep struct tags on one line per field with consistent alignment for readability.

## Anti-Patterns

### Using `map[string]interface{}` for config

Loses type safety, requires type assertions everywhere, and provides no validation. Always use typed structs.

### Scattering validation logic across the codebase

Validation belongs in the config package, not in service constructors. Services should receive already-validated config.

### Defining types separately from validation rules

Do not create a struct in one package and validation rules in another. The `validate` tags keep rules co-located with the type definition.

### Using exported global variables for config

Global `var Config AppConfig` creates hidden dependencies and makes testing difficult. Pass config explicitly via constructors.

## Related Patterns

- **[config-environment](./core-sdk-go.config-environment.md)** -- loading config values from environment variables using struct tags.
- **[config-loading](./core-sdk-go.config-loading.md)** -- layered config loading pipeline that populates these structs.
- **[config-secrets](./core-sdk-go.config-secrets.md)** -- the `Secret` type used for sensitive fields like `Password`.

## Testing

```go
package config_test

import (
	"testing"

	"yourmodule/config"
)

func TestAppConfig_Validate_ValidConfig(t *testing.T) {
	cfg := config.CreateTestConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
}

func TestAppConfig_Validate_InvalidPort(t *testing.T) {
	cfg := config.CreateTestConfig()
	cfg.Server.Port = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for port=0")
	}
}

func TestAppConfig_Validate_InvalidLogLevel(t *testing.T) {
	cfg := config.CreateTestConfig()
	cfg.Logging.Level = "verbose"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for invalid log level")
	}
}

func TestAppConfig_Validate_MissingDatabaseName(t *testing.T) {
	cfg := config.CreateTestConfig()
	cfg.Database.Name = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for empty database name")
	}
}

func TestAppConfig_ServiceCfg(t *testing.T) {
	cfg := config.CreateTestConfig()
	sc := cfg.ServiceCfg()
	if sc.Database.Host != cfg.Database.Host {
		t.Fatal("ServiceCfg should carry the same database config")
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
