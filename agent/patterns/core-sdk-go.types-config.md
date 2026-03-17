# Pattern: Configuration and Multi-Environment Types

**Namespace**: core-sdk-go
**Category**: Type System
**Created**: 2026-03-17
**Status**: Active

---

## Overview

Defines how to model, load, validate, and manage configuration in Go across multiple environments (development, staging, production). Combines struct definitions with struct tags, environment-specific embedding, build tags for compile-time selection, and runtime config file merging. This single pattern replaces both `types-config` and `config-multi-env` from the TypeScript core-sdk, unifying type definitions and multi-environment strategy into one Go-idiomatic approach.

## Problem

Configuration management in Go often devolves into one of several failure modes:

- **Flat env-var soup**: Dozens of `os.Getenv` calls scattered throughout code with no structure, no validation, and no defaults.
- **Single config struct**: One struct tries to serve all environments, leading to `if env == "production"` checks deep in business logic.
- **No validation**: Missing required values are discovered at runtime (often in production) rather than at startup.
- **No inheritance**: Each environment file duplicates the entire configuration, making it easy for shared values to drift.

## Solution

Use a layered configuration approach:

1. **Strongly-typed config structs** with struct tags for multiple sources (env vars, YAML, JSON).
2. **Base + override pattern**: A `BaseConfig` holds defaults shared across all environments; environment-specific configs embed it and override selectively.
3. **Build tags** for compile-time environment selection when config must be baked into the binary.
4. **Runtime file merging** for config-file-based environments (base.yaml + production.yaml).
5. **Validation at load time** using a `Validate()` method so invalid configs fail fast at startup.

## Implementation

### Project Structure

```
config/
  config.go            # BaseConfig, AppConfig, Validate()
  config_dev.go        # //go:build !production && !staging
  config_staging.go    # //go:build staging
  config_prod.go       # //go:build production
  loader.go            # File-based loader with merge logic
  loader_test.go
configs/
  base.yaml            # Shared defaults
  development.yaml     # Dev overrides
  staging.yaml         # Staging overrides
  production.yaml      # Production overrides
```

### Config Struct Definitions

```go
package config

import (
	"fmt"
	"time"
)

// DatabaseConfig holds database connection parameters.
type DatabaseConfig struct {
	Host            string        `yaml:"host" env:"DB_HOST"`
	Port            int           `yaml:"port" env:"DB_PORT"`
	Name            string        `yaml:"name" env:"DB_NAME"`
	User            string        `yaml:"user" env:"DB_USER"`
	Password        string        `yaml:"password" env:"DB_PASSWORD"`
	MaxOpenConns    int           `yaml:"max_open_conns" env:"DB_MAX_OPEN_CONNS"`
	MaxIdleConns    int           `yaml:"max_idle_conns" env:"DB_MAX_IDLE_CONNS"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" env:"DB_CONN_MAX_LIFETIME"`
	SSLMode         string        `yaml:"ssl_mode" env:"DB_SSL_MODE"`
}

// ServerConfig holds HTTP server parameters.
type ServerConfig struct {
	Host         string        `yaml:"host" env:"SERVER_HOST"`
	Port         int           `yaml:"port" env:"SERVER_PORT"`
	ReadTimeout  time.Duration `yaml:"read_timeout" env:"SERVER_READ_TIMEOUT"`
	WriteTimeout time.Duration `yaml:"write_timeout" env:"SERVER_WRITE_TIMEOUT"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" env:"SERVER_IDLE_TIMEOUT"`
}

// LogConfig holds logging parameters.
type LogConfig struct {
	Level  string `yaml:"level" env:"LOG_LEVEL"`
	Format string `yaml:"format" env:"LOG_FORMAT"` // "json" or "text"
}

// AuthConfig holds authentication parameters.
type AuthConfig struct {
	JWTSecret     string        `yaml:"jwt_secret" env:"AUTH_JWT_SECRET"`
	TokenExpiry   time.Duration `yaml:"token_expiry" env:"AUTH_TOKEN_EXPIRY"`
	RefreshExpiry time.Duration `yaml:"refresh_expiry" env:"AUTH_REFRESH_EXPIRY"`
}

// AppConfig is the top-level configuration struct that composes all
// sub-configs. It represents the complete, validated configuration
// for the application.
type AppConfig struct {
	Environment string         `yaml:"environment" env:"APP_ENV"`
	Debug       bool           `yaml:"debug" env:"APP_DEBUG"`
	Database    DatabaseConfig `yaml:"database"`
	Server      ServerConfig   `yaml:"server"`
	Log         LogConfig      `yaml:"log"`
	Auth        AuthConfig     `yaml:"auth"`
}
```

### Config Validation

```go
package config

import (
	"fmt"
	"strings"
)

// Validate checks all required fields and value constraints. It returns
// an error describing all problems found, not just the first.
func (c *AppConfig) Validate() error {
	var errs []string

	// Environment
	validEnvs := map[string]bool{
		"development": true,
		"staging":     true,
		"production":  true,
	}
	if !validEnvs[c.Environment] {
		errs = append(errs, fmt.Sprintf(
			"environment must be one of [development, staging, production], got %q",
			c.Environment,
		))
	}

	// Database
	if c.Database.Host == "" {
		errs = append(errs, "database.host is required")
	}
	if c.Database.Port <= 0 || c.Database.Port > 65535 {
		errs = append(errs, fmt.Sprintf("database.port must be 1-65535, got %d", c.Database.Port))
	}
	if c.Database.Name == "" {
		errs = append(errs, "database.name is required")
	}
	if c.Database.User == "" {
		errs = append(errs, "database.user is required")
	}

	// Server
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		errs = append(errs, fmt.Sprintf("server.port must be 1-65535, got %d", c.Server.Port))
	}

	// Auth - required in non-dev environments
	if c.Environment != "development" {
		if c.Auth.JWTSecret == "" {
			errs = append(errs, "auth.jwt_secret is required in non-development environments")
		}
		if len(c.Auth.JWTSecret) < 32 {
			errs = append(errs, "auth.jwt_secret must be at least 32 characters")
		}
	}

	// Log
	validLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if c.Log.Level != "" && !validLevels[c.Log.Level] {
		errs = append(errs, fmt.Sprintf(
			"log.level must be one of [debug, info, warn, error], got %q",
			c.Log.Level,
		))
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
```

### Defaults Function

```go
package config

import "time"

// Defaults returns an AppConfig populated with sensible default values.
// These serve as the "base" layer that environment-specific configs override.
func Defaults() AppConfig {
	return AppConfig{
		Environment: "development",
		Debug:       false,
		Database: DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
			SSLMode:         "disable",
		},
		Server: ServerConfig{
			Host:         "0.0.0.0",
			Port:         8080,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},
		Auth: AuthConfig{
			TokenExpiry:   1 * time.Hour,
			RefreshExpiry: 7 * 24 * time.Hour,
		},
	}
}
```

### Build Tags for Compile-Time Environment Selection

#### Development (default)

```go
//go:build !production && !staging

package config

// EnvDefaults returns environment-specific overrides for development.
func EnvDefaults() AppConfig {
	cfg := Defaults()
	cfg.Environment = "development"
	cfg.Debug = true
	cfg.Log.Level = "debug"
	cfg.Log.Format = "text"
	cfg.Database.SSLMode = "disable"
	return cfg
}
```

#### Staging

```go
//go:build staging

package config

// EnvDefaults returns environment-specific overrides for staging.
func EnvDefaults() AppConfig {
	cfg := Defaults()
	cfg.Environment = "staging"
	cfg.Debug = false
	cfg.Log.Level = "info"
	cfg.Database.SSLMode = "require"
	cfg.Database.MaxOpenConns = 50
	cfg.Server.Port = 8080
	return cfg
}
```

#### Production

```go
//go:build production

package config

// EnvDefaults returns environment-specific overrides for production.
func EnvDefaults() AppConfig {
	cfg := Defaults()
	cfg.Environment = "production"
	cfg.Debug = false
	cfg.Log.Level = "warn"
	cfg.Database.SSLMode = "verify-full"
	cfg.Database.MaxOpenConns = 100
	cfg.Database.MaxIdleConns = 25
	cfg.Server.ReadTimeout = 30 * time.Second
	cfg.Server.WriteTimeout = 30 * time.Second
	return cfg
}
```

Building with tags:

```bash
# Development (default, no tag needed)
go build ./cmd/server

# Staging
go build -tags staging ./cmd/server

# Production
go build -tags production ./cmd/server
```

### Runtime File-Based Config Loading with Merge

```go
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Load reads configuration from YAML files using a base + override strategy.
// It loads base.yaml first, then merges the environment-specific file on top.
// Environment variables take final precedence.
func Load(configDir, env string) (*AppConfig, error) {
	// Start with compiled defaults.
	cfg := Defaults()

	// Layer 1: base.yaml (shared across all environments).
	basePath := filepath.Join(configDir, "base.yaml")
	if err := mergeFromFile(&cfg, basePath); err != nil {
		return nil, fmt.Errorf("loading base config: %w", err)
	}

	// Layer 2: environment-specific file (e.g., production.yaml).
	envPath := filepath.Join(configDir, env+".yaml")
	if err := mergeFromFile(&cfg, envPath); err != nil {
		return nil, fmt.Errorf("loading %s config: %w", env, err)
	}

	// Layer 3: environment variables override everything.
	applyEnvOverrides(&cfg)

	// Validate the final merged config.
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// mergeFromFile reads a YAML file and unmarshals it onto the existing config.
// Because yaml.Unmarshal only sets fields present in the file, absent fields
// retain their previous values -- this is the merge behavior.
func mergeFromFile(cfg *AppConfig, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Missing file is OK; defaults remain.
		}
		return fmt.Errorf("reading %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	return nil
}

// applyEnvOverrides reads environment variables and applies them over
// the current config. Only non-empty env vars override.
func applyEnvOverrides(cfg *AppConfig) {
	if v := os.Getenv("APP_ENV"); v != "" {
		cfg.Environment = v
	}
	if v := os.Getenv("APP_DEBUG"); v == "true" {
		cfg.Debug = true
	}
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("AUTH_JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}
	if v := os.Getenv("SERVER_PORT"); v != "" {
		// Parse int from string; omitted for brevity.
		// Use strconv.Atoi in production code.
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
}
```

### Example YAML Config Files

#### configs/base.yaml

```yaml
# Shared defaults across all environments.
environment: development

database:
  host: localhost
  port: 5432
  name: myapp
  user: myapp
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 5m
  ssl_mode: disable

server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 15s
  write_timeout: 15s
  idle_timeout: 60s

log:
  level: info
  format: json

auth:
  token_expiry: 1h
  refresh_expiry: 168h  # 7 days
```

#### configs/production.yaml

```yaml
# Production overrides. Only fields that differ from base.yaml.
environment: production

database:
  host: db.internal.prod.example.com
  port: 5432
  name: myapp_prod
  user: myapp_prod
  max_open_conns: 100
  max_idle_conns: 25
  ssl_mode: verify-full
  # password comes from env var DB_PASSWORD

server:
  read_timeout: 30s
  write_timeout: 30s

log:
  level: warn

auth:
  # jwt_secret comes from env var AUTH_JWT_SECRET
  token_expiry: 30m
  refresh_expiry: 72h
```

#### configs/staging.yaml

```yaml
environment: staging

database:
  host: db.internal.staging.example.com
  name: myapp_staging
  user: myapp_staging
  max_open_conns: 50
  ssl_mode: require

log:
  level: info
```

### Application Bootstrap

```go
package main

import (
	"fmt"
	"log"
	"os"

	"yourmodule/config"
)

func main() {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	cfg, err := config.Load("configs", env)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	fmt.Printf("Starting %s server on %s:%d\n",
		cfg.Environment, cfg.Server.Host, cfg.Server.Port)

	// Pass cfg to constructors via dependency injection.
	// db, err := database.Connect(cfg.Database)
	// server := api.NewServer(cfg.Server, db)
	// server.ListenAndServe()
}
```

## Benefits

### 1. Layered Override Strategy
The base + environment + env-var layering means shared config is defined once, environments only specify differences, and secrets come from environment variables. No duplication.

### 2. Fail-Fast Validation
Calling `Validate()` at startup surfaces all configuration problems immediately with clear, aggregated error messages -- not one at a time, deep in a request handler.

### 3. Compile-Time Environment Locking
Build tags allow baking environment defaults into the binary. The production binary cannot accidentally run with development settings even if config files are missing.

### 4. Type Safety Throughout
Struct fields with proper Go types (`time.Duration`, `int`, `bool`) replace stringly-typed env vars. The compiler catches type mismatches.

### 5. Testable Configuration
Config is a plain struct -- tests can construct it directly without files or env vars, making configuration-dependent code easy to test.

## Best Practices

- **Secrets always via env vars**: Never put passwords, API keys, or JWT secrets in YAML files. Use the env-var override layer.
- **Validate at startup, not at use**: Call `Validate()` once during bootstrap. Downstream code can trust the config is valid.
- **Prefer composition over nesting**: Keep config structs shallow (2 levels max). Deep nesting makes YAML files hard to read.
- **Use `time.Duration` in structs**: YAML `5m`, `30s` parse naturally into `time.Duration` via the yaml.v3 library.
- **Document required vs optional fields**: Use comments in the base.yaml to indicate which fields must be overridden per environment.

## Anti-Patterns

### 1. Global Config Variable

```go
// BAD: Package-level mutable global, hard to test, invisible dependency.
var Config AppConfig

func init() {
    Config = loadConfig()
}

// GOOD: Load once in main, pass via dependency injection.
func main() {
    cfg, err := config.Load("configs", env)
    if err != nil {
        log.Fatal(err)
    }
    svc := service.New(cfg.Database)
}
```

### 2. Scattering os.Getenv Calls

```go
// BAD: Env vars read deep in business logic, untestable.
func (s *Service) Connect() error {
    host := os.Getenv("DB_HOST") // hidden dependency
    // ...
}

// GOOD: Config injected as a struct.
func (s *Service) Connect(cfg config.DatabaseConfig) error {
    // host is cfg.Host -- explicit, testable
}
```

### 3. One Giant Config File Per Environment

```yaml
# BAD: production.yaml duplicates everything from base.yaml.
# If base.yaml changes a default, production.yaml still has the old value.
environment: production
database:
  host: db.prod.example.com
  port: 5432             # duplicated from base
  name: myapp_prod
  user: myapp_prod
  max_open_conns: 100
  max_idle_conns: 25     # duplicated from base
  conn_max_lifetime: 5m  # duplicated from base
  ssl_mode: verify-full
server:
  host: 0.0.0.0          # duplicated from base
  port: 8080             # duplicated from base
  # ... everything duplicated ...

# GOOD: production.yaml only overrides what differs.
environment: production
database:
  host: db.prod.example.com
  name: myapp_prod
  user: myapp_prod
  max_open_conns: 100
  ssl_mode: verify-full
```

### 4. Skipping Validation

```go
// BAD: Config loaded but never validated. Empty DB host discovered
// 10 minutes later when the first request hits the database.
cfg, _ := config.Load("configs", env)

// GOOD: Load includes Validate(); errors are returned.
cfg, err := config.Load("configs", env)
if err != nil {
    log.Fatalf("config error: %v", err)
}
```

## Related Patterns

- **[core-sdk-go.types-shared](./core-sdk-go.types-shared.md)**: Config values like database host could use named types for additional safety, though this is typically unnecessary for config.
- **[core-sdk-go.types-error](./core-sdk-go.types-error.md)**: Config validation errors should use `ValidationError` from the error type system for consistent error handling.

## Testing

### Testing Config Validation

```go
package config_test

import (
	"strings"
	"testing"

	"yourmodule/config"
)

func TestValidate_ValidConfig(t *testing.T) {
	cfg := config.Defaults()
	cfg.Environment = "development"
	cfg.Database.Host = "localhost"
	cfg.Database.Port = 5432
	cfg.Database.Name = "testdb"
	cfg.Database.User = "testuser"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestValidate_MissingRequiredFields(t *testing.T) {
	cfg := config.AppConfig{
		Environment: "production",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for empty config")
	}

	msg := err.Error()
	if !strings.Contains(msg, "database.host is required") {
		t.Errorf("expected database.host error, got: %s", msg)
	}
	if !strings.Contains(msg, "auth.jwt_secret is required") {
		t.Errorf("expected auth.jwt_secret error in production, got: %s", msg)
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	cfg := config.Defaults()
	cfg.Database.Host = "localhost"
	cfg.Database.Name = "testdb"
	cfg.Database.User = "testuser"
	cfg.Database.Port = 99999

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for invalid port")
	}
	if !strings.Contains(err.Error(), "database.port") {
		t.Errorf("expected port error, got: %v", err)
	}
}
```

### Testing Config Loading and Merge

```go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"yourmodule/config"
)

func TestLoad_MergesBaseAndEnv(t *testing.T) {
	// Create temporary config directory.
	dir := t.TempDir()

	// Write base.yaml.
	base := []byte(`
environment: development
database:
  host: localhost
  port: 5432
  name: myapp
  user: myapp
server:
  port: 8080
log:
  level: info
`)
	os.WriteFile(filepath.Join(dir, "base.yaml"), base, 0644)

	// Write production.yaml with overrides only.
	prod := []byte(`
environment: production
database:
  host: db.prod.example.com
  name: myapp_prod
  user: myapp_prod
log:
  level: warn
`)
	os.WriteFile(filepath.Join(dir, "production.yaml"), prod, 0644)

	// Set required env vars for production.
	t.Setenv("AUTH_JWT_SECRET", "a-very-long-secret-that-is-at-least-32-chars")
	t.Setenv("DB_PASSWORD", "secret")

	cfg, err := config.Load(dir, "production")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Overridden fields.
	if cfg.Environment != "production" {
		t.Errorf("expected production, got %s", cfg.Environment)
	}
	if cfg.Database.Host != "db.prod.example.com" {
		t.Errorf("expected prod host, got %s", cfg.Database.Host)
	}
	if cfg.Log.Level != "warn" {
		t.Errorf("expected warn, got %s", cfg.Log.Level)
	}

	// Inherited fields from base.
	if cfg.Database.Port != 5432 {
		t.Errorf("expected port 5432 from base, got %d", cfg.Database.Port)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected server port 8080 from base, got %d", cfg.Server.Port)
	}
}

func TestLoad_EnvVarOverridesFile(t *testing.T) {
	dir := t.TempDir()

	base := []byte(`
environment: development
database:
  host: localhost
  port: 5432
  name: myapp
  user: myapp
`)
	os.WriteFile(filepath.Join(dir, "base.yaml"), base, 0644)

	// Env var overrides file value.
	t.Setenv("DB_HOST", "override.example.com")

	cfg, err := config.Load(dir, "development")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Database.Host != "override.example.com" {
		t.Errorf("expected env var override, got %s", cfg.Database.Host)
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
