# Pattern: Shared Domain Types

**Namespace**: core-sdk-go
**Category**: Type System
**Created**: 2026-03-17
**Status**: Active

---

## Overview

Defines how to model domain types in Go for type safety, clarity, and separation between internal domain models and external API representations. Uses Go's named types, constructor functions with validation, and explicit mapper functions to achieve goals similar to TypeScript's branded types and Zod schemas, but in an idiomatic Go style.

## Problem

Without discipline around type definitions, Go codebases suffer from:

- **Primitive obsession**: Passing raw `string` for user IDs, emails, and URLs leads to accidental misuse (e.g., passing an email where a user ID is expected).
- **Coupled representations**: Using the same struct for database rows, API responses, and internal logic creates tight coupling -- changing an API response format forces changes in business logic.
- **Missing validation**: Without constructor functions, invalid values (empty strings, malformed emails) propagate silently through the system.
- **Implicit contracts**: When every function accepts `string`, the type system provides no documentation about what values are expected.

## Solution

Apply three Go-idiomatic techniques:

1. **Named types** for domain identifiers and constrained values.
2. **Separate entity vs DTO structs** with explicit mapper functions between them.
3. **Constructor functions** that validate inputs at creation time, returning `(T, error)`.

## Implementation

### Project Structure

```
pkg/
  domain/
    types.go          # Named types (UserID, Email, etc.)
    user.go           # User entity + constructor
    user_dto.go       # UserDTO + mapper functions
  api/
    types.go          # API request/response types
```

### Named Types for Type Safety

```go
package domain

import (
	"fmt"
	"net/mail"
	"strings"

	"github.com/google/uuid"
)

// UserID is a domain identifier for users. It is a named type wrapping
// string, which prevents accidental interchange with other string-based IDs.
type UserID string

// NewUserID creates a UserID from a raw string, validating that it is
// a non-empty, valid UUID.
func NewUserID(raw string) (UserID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("user ID must not be empty")
	}
	if _, err := uuid.Parse(raw); err != nil {
		return "", fmt.Errorf("invalid user ID format: %w", err)
	}
	return UserID(raw), nil
}

// String implements the fmt.Stringer interface.
func (id UserID) String() string {
	return string(id)
}

// Email is a validated email address. Once constructed via NewEmail, the
// value is guaranteed to be well-formed.
type Email string

// NewEmail creates an Email from a raw string, validating RFC 5322 format.
func NewEmail(raw string) (Email, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("email must not be empty")
	}
	addr, err := mail.ParseAddress(raw)
	if err != nil {
		return "", fmt.Errorf("invalid email format %q: %w", raw, err)
	}
	return Email(addr.Address), nil
}

// String implements the fmt.Stringer interface.
func (e Email) String() string {
	return string(e)
}

// TenantID identifies a tenant in multi-tenant systems.
type TenantID string

// NewTenantID validates and creates a TenantID.
func NewTenantID(raw string) (TenantID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("tenant ID must not be empty")
	}
	return TenantID(raw), nil
}

// String implements the fmt.Stringer interface.
func (id TenantID) String() string {
	return string(id)
}
```

### Entity Structs (Domain Layer)

```go
package domain

import "time"

// User is the domain entity -- the single source of truth for user
// state within the application. It uses named types for all identifiers.
type User struct {
	ID        UserID
	TenantID  TenantID
	Email     Email
	Name      string
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewUser constructs a User with validated fields. Business rules
// (e.g., name length) are enforced here.
func NewUser(id UserID, tenantID TenantID, email Email, name string) (*User, error) {
	if name == "" {
		return nil, fmt.Errorf("user name must not be empty")
	}
	if len(name) > 256 {
		return nil, fmt.Errorf("user name must not exceed 256 characters")
	}
	now := time.Now().UTC()
	return &User{
		ID:        id,
		TenantID:  tenantID,
		Email:     email,
		Name:      name,
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
```

### DTO Structs (API Layer)

```go
package domain

// UserDTO is the data transfer object used for API serialization.
// It uses primitive types and struct tags for JSON encoding.
type UserDTO struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// UserToDTO converts a domain User entity to its API representation.
func UserToDTO(u *User) UserDTO {
	return UserDTO{
		ID:        u.ID.String(),
		TenantID:  u.TenantID.String(),
		Email:     u.Email.String(),
		Name:      u.Name,
		Active:    u.Active,
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
		UpdatedAt: u.UpdatedAt.Format(time.RFC3339),
	}
}

// UserFromDTO converts an API representation back to a domain User entity.
// Returns an error if any field fails validation.
func UserFromDTO(dto UserDTO) (*User, error) {
	id, err := NewUserID(dto.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid user DTO: %w", err)
	}
	tenantID, err := NewTenantID(dto.TenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid user DTO: %w", err)
	}
	email, err := NewEmail(dto.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid user DTO: %w", err)
	}

	createdAt, err := time.Parse(time.RFC3339, dto.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid created_at: %w", err)
	}
	updatedAt, err := time.Parse(time.RFC3339, dto.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid updated_at: %w", err)
	}

	return &User{
		ID:        id,
		TenantID:  tenantID,
		Email:     email,
		Name:      dto.Name,
		Active:    dto.Active,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

// UsersToDTO converts a slice of domain Users to DTOs.
func UsersToDTO(users []*User) []UserDTO {
	dtos := make([]UserDTO, len(users))
	for i, u := range users {
		dtos[i] = UserToDTO(u)
	}
	return dtos
}
```

### Stringer Interface for Debugging

```go
package domain

import "fmt"

// String provides a human-readable representation suitable for logging.
// Sensitive fields like Email are partially redacted.
func (u *User) String() string {
	return fmt.Sprintf("User{ID: %s, Name: %q, Active: %t}", u.ID, u.Name, u.Active)
}
```

## Benefits

### 1. Compile-Time Safety
Named types like `UserID` and `Email` prevent accidental interchange at compile time. A function accepting `UserID` will not compile if you pass an `Email`.

### 2. Validation at the Boundary
Constructor functions (e.g., `NewEmail`) enforce invariants once at creation. All downstream code can trust the value is valid without re-checking.

### 3. Decoupled Representations
Entity and DTO structs evolve independently. API format changes (e.g., renaming a JSON field) do not ripple into business logic.

### 4. Self-Documenting Code
Function signatures like `func GetUser(id UserID)` are immediately clear about what they accept, without needing comments.

## Best Practices

- **Always use constructor functions** for named types that have invariants (email format, UUID format, non-empty). Export the constructor, not a bare type cast.
- **Keep mappers next to the DTO** they convert, in the same package. This makes them easy to find and update together.
- **Implement `fmt.Stringer`** on all named types and entities to improve log output and debugging.
- **Use `time.Time` in entities, `string` in DTOs.** Parse and format at the boundary.
- **Prefer pointer receivers for entities** (`*User`) and value receivers for named types (`UserID`, `Email`) since named types are small.

## Anti-Patterns

### 1. Bare Type Casting Without Validation

```go
// BAD: Bypasses validation, allows empty or malformed values.
id := domain.UserID("not-a-uuid")

// GOOD: Always use the constructor.
id, err := domain.NewUserID("not-a-uuid")
if err != nil {
    // handle error
}
```

### 2. Using the Same Struct for Domain and API

```go
// BAD: Mixing JSON tags into domain logic couples layers.
type User struct {
    ID    string `json:"id" db:"user_id"`
    Email string `json:"email" db:"email"`
}

// GOOD: Separate structs with explicit mappers.
type User struct {    // domain
    ID    UserID
    Email Email
}
type UserDTO struct {  // API
    ID    string `json:"id"`
    Email string `json:"email"`
}
```

### 3. Exporting Unexported Constructor Side Effects

```go
// BAD: Returning a zero-value named type on error.
func NewEmail(raw string) Email {
    // silently returns "" on failure
    addr, _ := mail.ParseAddress(raw)
    if addr == nil {
        return ""
    }
    return Email(addr.Address)
}

// GOOD: Return (T, error) so the caller must handle failure.
func NewEmail(raw string) (Email, error) { ... }
```

## Related Patterns

- **[core-sdk-go.types-error](./core-sdk-go.types-error.md)**: Error types follow the same named-type philosophy, with custom types implementing the `error` interface.
- **[core-sdk-go.types-config](./core-sdk-go.types-config.md)**: Config types use struct embedding and tags, complementing the entity/DTO separation shown here.

## Testing

### Unit Tests for Named Types

```go
package domain_test

import (
	"testing"

	"yourmodule/pkg/domain"
)

func TestNewEmail_Valid(t *testing.T) {
	email, err := domain.NewEmail("alice@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if email.String() != "alice@example.com" {
		t.Errorf("got %q, want %q", email, "alice@example.com")
	}
}

func TestNewEmail_Invalid(t *testing.T) {
	cases := []string{"", "not-an-email", "@missing.local", "   "}
	for _, tc := range cases {
		_, err := domain.NewEmail(tc)
		if err == nil {
			t.Errorf("expected error for %q, got nil", tc)
		}
	}
}

func TestNewUserID_Valid(t *testing.T) {
	id, err := domain.NewUserID("550e8400-e29b-41d4-a716-446655440000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.String() != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("unexpected ID value: %s", id)
	}
}

func TestUserToDTO_RoundTrip(t *testing.T) {
	id, _ := domain.NewUserID("550e8400-e29b-41d4-a716-446655440000")
	tid, _ := domain.NewTenantID("tenant-1")
	email, _ := domain.NewEmail("alice@example.com")
	user, err := domain.NewUser(id, tid, email, "Alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dto := domain.UserToDTO(user)
	restored, err := domain.UserFromDTO(dto)
	if err != nil {
		t.Fatalf("round-trip failed: %v", err)
	}

	if restored.ID != user.ID {
		t.Errorf("ID mismatch: %s != %s", restored.ID, user.ID)
	}
	if restored.Email != user.Email {
		t.Errorf("Email mismatch: %s != %s", restored.Email, user.Email)
	}
}
```

### Comparison with TypeScript Branded Types

In TypeScript core-sdk, branded types look like:

```typescript
type UserID = string & { __brand: "UserID" };
```

Go named types (`type UserID string`) provide similar but weaker guarantees:

| Feature | TypeScript Branded | Go Named Type |
|---|---|---|
| Compile-time distinction | Yes | Yes |
| Prevents accidental `string` use | Yes (with brand) | Yes (different type) |
| Runtime overhead | None | None |
| Explicit cast required | Yes | Yes (`UserID(s)`) |
| Validation enforcement | Manual (via factory) | Manual (via constructor) |
| Structural subtyping bypass | Brand prevents it | Named typing prevents it |

The key difference: TypeScript branded types exist only at compile time and vanish at runtime. Go named types are real distinct types that also affect runtime (e.g., you cannot pass a `UserID` to a function expecting `string` without an explicit conversion). This makes Go's approach slightly more rigid and arguably safer.

---

**Status**: Active
**Compatibility**: Go 1.21+
