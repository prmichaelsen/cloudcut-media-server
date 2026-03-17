# Pattern: Config Secrets

**Namespace**: core-sdk-go
**Category**: Configuration
**Created**: 2026-03-17
**Status**: Active

---

## Overview

The Config Secrets pattern provides a `Secret` type that wraps sensitive configuration values (passwords, API keys, tokens) and prevents their accidental exposure through logging, serialization, or debugging output. The type implements `fmt.Stringer`, `json.Marshaler`, and `encoding.TextMarshaler` to always return a redacted placeholder, while providing an explicit `Value()` method for intentional access.

## Problem

Sensitive configuration values are regular strings that can be accidentally leaked through:

- `fmt.Printf("%+v", config)` in debug logging
- JSON marshaling in health endpoints or error responses
- Stack traces and error messages
- Structured logging (zerolog, zap) that serializes struct fields

In the TypeScript core-sdk, passwords are plain strings with no protection. Go's interface system allows us to build a type that is safe by default.

## Solution

Define a `Secret` type that:

1. Stores the actual value in an unexported field
2. Returns `[REDACTED]` from `String()`, `MarshalJSON()`, and `MarshalText()`
3. Exposes the real value only through an explicit `Value()` method
4. Supports loading from environment variables and vault clients

## Implementation

### The Secret Type

```go
package config

import (
	"encoding/json"
	"fmt"
)

// Secret wraps a sensitive string value, preventing accidental exposure
// in logs, JSON output, and fmt formatting.
type Secret struct {
	value string
}

const redacted = "[REDACTED]"

// NewSecret creates a Secret from a plaintext value.
func NewSecret(value string) Secret {
	return Secret{value: value}
}

// Value returns the actual secret value. Use intentionally and sparingly.
func (s Secret) Value() string {
	return s.value
}

// IsEmpty reports whether the secret has a zero-length value.
func (s Secret) IsEmpty() bool {
	return s.value == ""
}

// String implements fmt.Stringer. Always returns "[REDACTED]".
func (s Secret) String() string {
	return redacted
}

// GoString implements fmt.GoStringer for %#v formatting.
func (s Secret) GoString() string {
	return fmt.Sprintf("config.Secret{%s}", redacted)
}

// MarshalJSON implements json.Marshaler. Returns "[REDACTED]" as a JSON string.
func (s Secret) MarshalJSON() ([]byte, error) {
	return json.Marshal(redacted)
}

// UnmarshalJSON implements json.Unmarshaler. Reads the actual value from JSON.
func (s *Secret) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	s.value = v
	return nil
}

// MarshalText implements encoding.TextMarshaler. Returns "[REDACTED]".
// This is used by YAML marshalers and structured loggers.
func (s Secret) MarshalText() ([]byte, error) {
	return []byte(redacted), nil
}

// UnmarshalText implements encoding.TextUnmarshaler. Reads the actual value.
func (s *Secret) UnmarshalText(text []byte) error {
	s.value = string(text)
	return nil
}

// MarshalYAML returns the redacted placeholder for YAML serialization.
func (s Secret) MarshalYAML() (interface{}, error) {
	return redacted, nil
}

// UnmarshalYAML reads the actual value from YAML.
func (s *Secret) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var v string
	if err := unmarshal(&v); err != nil {
		return err
	}
	s.value = v
	return nil
}
```

### Using Secret in Config Structs

```go
package config

// DatabaseConfig uses Secret for the password field.
type DatabaseConfig struct {
	Host     string `json:"host"     yaml:"host"     validate:"required"`
	Port     int    `json:"port"     yaml:"port"     validate:"required,min=1,max=65535"`
	Name     string `json:"name"     yaml:"name"     validate:"required"`
	User     string `json:"user"     yaml:"user"     validate:"required"`
	Password Secret `json:"password" yaml:"password" validate:"required"`
	SSL      bool   `json:"ssl"      yaml:"ssl"`
}

// APIConfig demonstrates multiple secret fields.
type APIConfig struct {
	BaseURL   string `json:"base_url"   validate:"required,url"`
	APIKey    Secret `json:"api_key"    validate:"required"`
	APISecret Secret `json:"api_secret" validate:"required"`
}
```

### How Redaction Works in Practice

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"yourmodule/config"
)

func main() {
	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "myapp",
		User:     "admin",
		Password: config.NewSecret("super-secret-password"),
	}

	// fmt.Println -- redacted via String()
	fmt.Println("Password:", cfg.Password)
	// Output: Password: [REDACTED]

	// fmt.Printf with %+v -- redacted via String()
	fmt.Printf("Config: %+v\n", cfg)
	// Output: Config: {Host:localhost Port:5432 Name:myapp User:admin Password:[REDACTED] SSL:false}

	// fmt.Printf with %#v -- redacted via GoString()
	fmt.Printf("Config: %#v\n", cfg.Password)
	// Output: Config: config.Secret{[REDACTED]}

	// JSON marshaling -- redacted via MarshalJSON()
	data, _ := json.MarshalIndent(cfg, "", "  ")
	fmt.Println(string(data))
	// Output:
	// {
	//   "host": "localhost",
	//   "port": 5432,
	//   "name": "myapp",
	//   "user": "admin",
	//   "password": "[REDACTED]",
	//   "ssl": false
	// }

	// Intentional access -- explicit Value() call
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		cfg.User,
		cfg.Password.Value(), // Explicit access
		cfg.Host,
		cfg.Port,
		cfg.Name,
	)
	log.Printf("connecting to database") // DSN not logged
	_ = dsn
}
```

### Loading Secrets from Environment Variables

```go
package config

import (
	"os"
)

// LoadSecretsFromEnv loads secret values from environment variables.
func LoadSecretsFromEnv(cfg *AppConfig) {
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.Database.Password = NewSecret(v)
	}
	if v := os.Getenv("API_KEY"); v != "" {
		cfg.API.APIKey = NewSecret(v)
	}
	if v := os.Getenv("API_SECRET"); v != "" {
		cfg.API.APISecret = NewSecret(v)
	}
}
```

### Integration with Vault Clients

```go
package config

import (
	"context"
	"fmt"
)

// VaultClient is an interface for secret backends (HashiCorp Vault, AWS Secrets Manager, etc.)
type VaultClient interface {
	GetSecret(ctx context.Context, path string) (string, error)
}

// LoadSecretsFromVault loads secrets from a vault backend.
func LoadSecretsFromVault(ctx context.Context, vault VaultClient, cfg *AppConfig) error {
	dbPass, err := vault.GetSecret(ctx, "database/password")
	if err != nil {
		return fmt.Errorf("loading database password: %w", err)
	}
	cfg.Database.Password = NewSecret(dbPass)

	apiKey, err := vault.GetSecret(ctx, "api/key")
	if err != nil {
		return fmt.Errorf("loading API key: %w", err)
	}
	cfg.API.APIKey = NewSecret(apiKey)

	return nil
}
```

### Custom Validator for Secret

```go
package config

import (
	"github.com/go-playground/validator/v10"
)

// RegisterSecretValidators adds custom validation for the Secret type.
func RegisterSecretValidators(v *validator.Validate) {
	v.RegisterCustomTypeFunc(func(field reflect.Value) interface{} {
		if s, ok := field.Interface().(Secret); ok {
			return s.Value()
		}
		return ""
	}, Secret{})
}

// Usage: the "required" tag on a Secret field now checks that Value() is non-empty.
```

### Integration with Structured Loggers

```go
package main

import (
	"os"

	"github.com/rs/zerolog"
	"yourmodule/config"
)

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg := config.DatabaseConfig{
		Host:     "db.example.com",
		Port:     5432,
		Name:     "myapp",
		User:     "admin",
		Password: config.NewSecret("hunter2"),
	}

	// zerolog uses MarshalText/MarshalJSON -- password is automatically redacted
	logger.Info().
		Str("host", cfg.Host).
		Int("port", cfg.Port).
		Str("password", cfg.Password.String()). // [REDACTED]
		Msg("database config loaded")
}
```

## Benefits

1. **Safe by default** -- all standard output paths (fmt, JSON, YAML, structured logs) return redacted values automatically.
2. **Explicit access** -- `Value()` makes secret access visible in code and easy to audit via grep.
3. **Zero runtime overhead** -- the wrapper is a single-field struct with no heap allocation beyond the underlying string.
4. **Drop-in replacement** -- works with JSON/YAML unmarshalers, config libraries, and validators without special handling.
5. **Auditability** -- searching the codebase for `.Value()` reveals every point where secrets are extracted.

## Best Practices

- Use `Secret` for every field that holds credentials, tokens, API keys, or other sensitive data.
- Never store the result of `Value()` in a variable longer than necessary. Extract, use, discard.
- Register the custom validator so `validate:"required"` works correctly on Secret fields.
- In code reviews, flag any `.Value()` call that feeds into a log statement or serialization path.
- Use the `VaultClient` interface pattern to abstract secret backends; swap implementations for testing.

## Anti-Patterns

### Calling Value() for logging

```go
// BAD: defeats the purpose of Secret
log.Printf("connecting with password: %s", cfg.Password.Value())

// GOOD: let String() handle it
log.Printf("connecting with password: %s", cfg.Password)
// Output: connecting with password: [REDACTED]
```

### Storing Value() in a plain string field

```go
// BAD: secret escapes the wrapper
type Connection struct {
    password string // plain string, no redaction
}
conn := Connection{password: cfg.Password.Value()}

// GOOD: keep the Secret type
type Connection struct {
    password config.Secret
}
conn := Connection{password: cfg.Password}
```

### Using a pointer receiver for Secret methods

Secret methods use value receivers so that even copies retain redaction behavior. Using pointer receivers would allow nil secrets to panic.

## Related Patterns

- **[config-struct](./core-sdk-go.config-struct.md)** -- where `Secret` fields are used in config struct definitions.
- **[config-environment](./core-sdk-go.config-environment.md)** -- loading secret values from environment variables.
- **[config-loading](./core-sdk-go.config-loading.md)** -- the full pipeline where secrets are loaded as one layer.

## Testing

```go
package config_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"yourmodule/config"
)

func TestSecret_String_ReturnsRedacted(t *testing.T) {
	s := config.NewSecret("my-password")
	if s.String() != "[REDACTED]" {
		t.Errorf("expected [REDACTED], got %q", s.String())
	}
}

func TestSecret_Value_ReturnsActual(t *testing.T) {
	s := config.NewSecret("my-password")
	if s.Value() != "my-password" {
		t.Errorf("expected 'my-password', got %q", s.Value())
	}
}

func TestSecret_IsEmpty(t *testing.T) {
	empty := config.NewSecret("")
	if !empty.IsEmpty() {
		t.Error("expected IsEmpty() to be true for empty secret")
	}

	nonEmpty := config.NewSecret("value")
	if nonEmpty.IsEmpty() {
		t.Error("expected IsEmpty() to be false for non-empty secret")
	}
}

func TestSecret_MarshalJSON_Redacted(t *testing.T) {
	s := config.NewSecret("hunter2")
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `"[REDACTED]"` {
		t.Errorf("expected JSON \"[REDACTED]\", got %s", data)
	}
}

func TestSecret_UnmarshalJSON_ReadsValue(t *testing.T) {
	var s config.Secret
	if err := json.Unmarshal([]byte(`"my-secret"`), &s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Value() != "my-secret" {
		t.Errorf("expected 'my-secret', got %q", s.Value())
	}
}

func TestSecret_Fprintf_Redacted(t *testing.T) {
	s := config.NewSecret("hunter2")

	result := fmt.Sprintf("%s", s)
	if result != "[REDACTED]" {
		t.Errorf("%%s: expected [REDACTED], got %q", result)
	}

	result = fmt.Sprintf("%v", s)
	if result != "[REDACTED]" {
		t.Errorf("%%v: expected [REDACTED], got %q", result)
	}

	result = fmt.Sprintf("%#v", s)
	if result != "config.Secret{[REDACTED]}" {
		t.Errorf("%%#v: expected config.Secret{[REDACTED]}, got %q", result)
	}
}

func TestSecret_InStruct_JSONMarshal(t *testing.T) {
	type Config struct {
		Host     string        `json:"host"`
		Password config.Secret `json:"password"`
	}
	cfg := Config{Host: "localhost", Password: config.NewSecret("secret123")}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]string
	json.Unmarshal(data, &result)

	if result["password"] != "[REDACTED]" {
		t.Errorf("expected password to be [REDACTED] in JSON, got %q", result["password"])
	}
	if result["host"] != "localhost" {
		t.Errorf("expected host 'localhost', got %q", result["host"])
	}
}

func TestSecret_InStruct_JSONUnmarshal(t *testing.T) {
	type Config struct {
		Host     string        `json:"host"`
		Password config.Secret `json:"password"`
	}
	var cfg Config
	input := `{"host":"localhost","password":"secret123"}`
	if err := json.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Password.Value() != "secret123" {
		t.Errorf("expected password value 'secret123', got %q", cfg.Password.Value())
	}
}

func TestSecret_ZeroValue_IsEmpty(t *testing.T) {
	var s config.Secret
	if !s.IsEmpty() {
		t.Error("zero-value Secret should be empty")
	}
	if s.String() != "[REDACTED]" {
		t.Error("zero-value Secret should still return [REDACTED]")
	}
}
```

---

**Status**: Active
**Compatibility**: Go 1.21+
