# Task 15: Create Plugin Manifest Parser

**Status**: Not Started
**Milestone**: M5 - Plugin System Foundation
**Estimated Hours**: 3-4
**Priority**: High

---

## Objective

Implement a plugin manifest parser that reads and validates `plugin.yaml` files, ensuring plugins declare their capabilities correctly.

---

## Context

The plugin manifest (`plugin.yaml`) declares:
- Plugin metadata (name, version, author)
- Plugin type (go-plugin, wasm, container, script)
- Extension points (effects.video, effects.audio, etc.)
- Parameters and their schemas
- Activation events

**Design reference**: `agent/design/plugin-architecture-backend.md` § Plugin Manifest

---

## Steps

### 1. Define Manifest Schema

**Manifest structure**:

```yaml
name: vintage-film-effects
version: 1.0.0
type: go-plugin  # or wasm, container, script
runtime:
  path: ./plugin.so
  entrypoint: Effect

contributes:
  effects.video:
    - id: film-grain
      name: Film Grain
      parameters:
        - id: intensity
          type: float
          default: 0.5
          min: 0.0
          max: 1.0
      implementation:
        type: ffmpeg-filter
        template: "noise=alls={{intensity|mul:100}}:allf=t"
```

**Go types**:

```go
type PluginManifest struct {
    Name    string          `yaml:"name"`
    Version string          `yaml:"version"`
    Type    PluginType      `yaml:"type"`
    Runtime RuntimeConfig   `yaml:"runtime"`
    Contributes Contributions `yaml:"contributes"`
}

type RuntimeConfig struct {
    Path       string `yaml:"path"`
    Entrypoint string `yaml:"entrypoint"`
}

type Contributions struct {
    VideoEffects []VideoEffectContribution `yaml:"effects.video"`
    AudioEffects []AudioEffectContribution `yaml:"effects.audio"`
}

type VideoEffectContribution struct {
    ID             string            `yaml:"id"`
    Name           string            `yaml:"name"`
    Parameters     []ParameterSchema `yaml:"parameters"`
    Implementation Implementation    `yaml:"implementation"`
}

type ParameterSchema struct {
    ID      string      `yaml:"id"`
    Type    string      `yaml:"type"` // float, int, string, bool
    Default interface{} `yaml:"default"`
    Min     interface{} `yaml:"min,omitempty"`
    Max     interface{} `yaml:"max,omitempty"`
}

type Implementation struct {
    Type     string `yaml:"type"` // ffmpeg-filter, http-service, etc.
    Template string `yaml:"template,omitempty"`
    URL      string `yaml:"url,omitempty"`
}
```

**Action**: Define types in `internal/plugin/manifest.go`

### 2. Implement Manifest Parser

**Parser functions**:

```go
// ParseManifest reads and parses a plugin.yaml file
func ParseManifest(path string) (*PluginManifest, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read manifest: %w", err)
    }

    var manifest PluginManifest
    if err := yaml.Unmarshal(data, &manifest); err != nil {
        return nil, fmt.Errorf("parse manifest: %w", err)
    }

    if err := ValidateManifest(&manifest); err != nil {
        return nil, fmt.Errorf("validate manifest: %w", err)
    }

    return &manifest, nil
}
```

**Action**: Implement parser in `internal/plugin/manifest.go`

### 3. Implement Manifest Validation

**Validation rules**:
- Name is required and non-empty
- Version is valid semver (use semver library)
- Type is one of known PluginType values
- Runtime path exists (relative to plugin directory)
- All effect IDs are unique within plugin
- Parameter types are valid (float, int, string, bool)
- Min/max only present for numeric types
- Default values match parameter type

**Validator**:

```go
type ValidationErrors []ValidationError

type ValidationError struct {
    Field   string
    Message string
}

func ValidateManifest(m *PluginManifest) ValidationErrors {
    var errs ValidationErrors

    // Name
    if m.Name == "" {
        errs = append(errs, ValidationError{
            Field: "name",
            Message: "name is required",
        })
    }

    // Version (semver)
    if !isValidSemver(m.Version) {
        errs = append(errs, ValidationError{
            Field: "version",
            Message: fmt.Sprintf("invalid semver: %s", m.Version),
        })
    }

    // Type
    if !isValidPluginType(m.Type) {
        errs = append(errs, ValidationError{
            Field: "type",
            Message: fmt.Sprintf("invalid type: %s", m.Type),
        })
    }

    // Effects
    errs = append(errs, validateEffects(m.Contributes.VideoEffects)...)

    return errs
}
```

**Action**: Implement validation in `internal/plugin/manifest.go`

### 4. Add Helper Functions

**Helpers**:
- `isValidSemver(version string) bool` - Check semver validity
- `isValidPluginType(t PluginType) bool` - Check plugin type
- `isValidParameterType(t string) bool` - Check parameter type
- `validateParameterDefault(param ParameterSchema) error` - Check default matches type

**Action**: Implement helpers

### 5. Write Tests

**Test cases**:

```go
func TestParseManifest_Valid(t *testing.T)
func TestParseManifest_InvalidYAML(t *testing.T)
func TestParseManifest_MissingName(t *testing.T)
func TestParseManifest_InvalidSemver(t *testing.T)
func TestParseManifest_InvalidType(t *testing.T)
func TestParseManifest_InvalidParameterType(t *testing.T)
func TestParseManifest_MismatchedDefaultType(t *testing.T)
func TestValidateManifest_DuplicateEffectIDs(t *testing.T)
```

**Test fixtures**:
- `testdata/manifests/valid.yaml`
- `testdata/manifests/invalid-*.yaml` for each error case

**Action**: Implement tests in `internal/plugin/manifest_test.go`

### 6. Add Manifest Examples

**Action**: Create sample manifests in `plugins/examples/`
- `plugins/examples/go-plugin/plugin.yaml`
- `plugins/examples/wasm-plugin/plugin.yaml` (future)
- `plugins/examples/container-plugin/plugin.yaml` (future)

---

## Verification

- [ ] `internal/plugin/manifest.go` created
- [ ] PluginManifest types defined
- [ ] ParseManifest() reads and parses YAML
- [ ] ValidateManifest() catches all error cases
- [ ] Invalid semver rejected
- [ ] Invalid plugin type rejected
- [ ] Invalid parameter types rejected
- [ ] Default value type mismatches caught
- [ ] All unit tests passing
- [ ] Sample manifests created

---

## Definition of Done

- Manifest parser implemented
- Validation logic complete
- Tests written and passing
- Example manifests created
- Committed to repository

---

## Dependencies

**Blocking**:
- Task 14 (Plugin registry needs manifest types)

**Required Libraries**:
- `gopkg.in/yaml.v3` for YAML parsing
- `github.com/Masterminds/semver/v3` for semver validation

---

## Notes

- Keep manifest schema extensible (allow unknown fields for future)
- Consider JSON Schema validation for manifest (overkill for MVP)
- Document manifest schema in `plugins/README.md`
- Add manifest schema version field for future evolution

---

## Related Documents

- [`agent/design/plugin-architecture-backend.md`](../../design/plugin-architecture-backend.md) § Plugin Manifest

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
