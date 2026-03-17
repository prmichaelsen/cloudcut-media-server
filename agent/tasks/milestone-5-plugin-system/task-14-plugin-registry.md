# Task 14: Implement Plugin Registry

**Status**: Not Started
**Milestone**: M5 - Plugin System Foundation
**Estimated Hours**: 4-6
**Priority**: High

---

## Objective

Implement a thread-safe plugin registry that discovers, loads, and manages server-side plugins with support for multiple plugin types (Go plugins initially).

---

## Context

The plugin registry is the core of the plugin system. It provides:
- **Discovery**: Scans `plugins/` directory for plugin manifests
- **Loading**: Loads plugins based on manifest type (Go .so, WASM, containers)
- **Resolution**: Retrieves plugins by ID during EDL processing
- **Lifecycle**: Manages plugin activation/deactivation

**Design reference**: `agent/design/plugin-architecture-backend.md` § Plugin Registry

---

## Steps

### 1. Create Plugin Package Structure

**Action**: Create package structure
```
internal/plugin/
├── registry.go          # Main registry implementation
├── plugin.go            # Plugin interface definitions
├── loader.go            # Plugin loading logic
├── manifest.go          # Manifest types (created in Task 15)
└── registry_test.go     # Tests
```

### 2. Define Plugin Interfaces

**Core interfaces**:

```go
// Plugin is the base interface all plugins must implement
type Plugin interface {
    ID() string
    Name() string
    Version() string
    Type() PluginType
}

// VideoEffectPlugin extends Plugin for video effects
type VideoEffectPlugin interface {
    Plugin
    BuildFFmpegFilter(params map[string]interface{}) (string, error)
    GetParameters() []EffectParameter
}

// EffectParameter describes a plugin parameter
type EffectParameter struct {
    ID      string
    Type    string // "float", "int", "string", "bool"
    Default interface{}
    Min     interface{} // for numeric types
    Max     interface{}
}

// PluginType enum
type PluginType string
const (
    PluginTypeGoPlugin   PluginType = "go-plugin"
    PluginTypeWASM       PluginType = "wasm"
    PluginTypeContainer  PluginType = "container"
    PluginTypeScript     PluginType = "script"
)
```

**Action**: Define interfaces in `internal/plugin/plugin.go`

### 3. Implement Plugin Registry

**Registry structure**:

```go
type PluginRegistry struct {
    mu              sync.RWMutex
    plugins         map[string]Plugin
    videoEffects    map[string]VideoEffectPlugin
    audioEffects    map[string]AudioEffectPlugin // future
    exporters       map[string]ExportPlugin      // future
}

func NewPluginRegistry() *PluginRegistry {
    return &PluginRegistry{
        plugins:      make(map[string]Plugin),
        videoEffects: make(map[string]VideoEffectPlugin),
    }
}
```

**Methods to implement**:

```go
// LoadPlugins discovers and loads all plugins from directory
func (r *PluginRegistry) LoadPlugins(pluginDir string) error

// Register adds a plugin to the registry
func (r *PluginRegistry) Register(plugin Plugin) error

// GetEffect retrieves a video effect plugin by ID
func (r *PluginRegistry) GetEffect(id string) (VideoEffectPlugin, error)

// ListEffects returns all registered video effects
func (r *PluginRegistry) ListEffects() []VideoEffectPlugin

// Unload removes a plugin from the registry
func (r *PluginRegistry) Unload(id string) error
```

**Action**: Implement `internal/plugin/registry.go`

### 4. Implement Plugin Discovery

**Discovery logic**:
1. Scan `pluginDir` for subdirectories
2. Check each subdirectory for `plugin.yaml`
3. Parse manifest (defer to Task 15)
4. Load plugin based on type
5. Register plugin

**Error handling**:
- Invalid manifest → log warning, skip plugin
- Load failure → log error, continue with other plugins
- Duplicate plugin IDs → return error

**Action**: Implement `LoadPlugins()` method

### 5. Add Thread Safety

**Concurrency considerations**:
- Multiple goroutines may query registry during rendering
- Plugin loading happens once at startup (or on hot reload later)
- Use RWMutex: write lock for registration, read lock for queries

**Action**: Add mutex protection to all registry methods

### 6. Add Logging

**Log events**:
- Plugin discovery started (how many found)
- Plugin loaded successfully (ID, version, type)
- Plugin load failed (ID, reason)
- Plugin registered (ID, extension points)

**Action**: Add structured logging with context

### 7. Write Tests

**Test cases**:

```go
func TestPluginRegistry_RegisterAndRetrieve(t *testing.T)
func TestPluginRegistry_DuplicateID(t *testing.T)
func TestPluginRegistry_UnknownPlugin(t *testing.T)
func TestPluginRegistry_ThreadSafety(t *testing.T) // concurrent reads
func TestPluginRegistry_LoadPlugins(t *testing.T) // with test fixtures
```

**Test fixtures**:
- Create `testdata/plugins/valid-plugin/` with sample plugin.yaml
- Create `testdata/plugins/invalid-plugin/` with malformed manifest

**Action**: Implement tests in `internal/plugin/registry_test.go`

---

## Verification

- [ ] `internal/plugin/registry.go` created
- [ ] Plugin and VideoEffectPlugin interfaces defined
- [ ] PluginRegistry struct with thread-safe methods
- [ ] LoadPlugins() discovers plugins from directory
- [ ] Register() adds plugins to registry
- [ ] GetEffect() retrieves plugins by ID
- [ ] Unknown plugin returns appropriate error
- [ ] Duplicate plugin ID returns error
- [ ] Thread safety tests pass
- [ ] All unit tests passing
- [ ] Logging provides visibility into plugin loading

---

## Definition of Done

- Plugin registry implemented with thread safety
- Plugin interfaces defined
- Discovery and registration logic complete
- Tests written and passing
- Logging added
- Committed to repository

---

## Dependencies

**Blocking**:
- None (foundational task)

**Downstream**:
- Task 15 (Manifest parser) will provide manifest types
- Task 16 (Go plugin loader) will provide loading implementation

---

## Notes

- Keep registry simple initially (no hot reload, no versioning)
- Consider plugin activation/deactivation lifecycle for future
- Plugin panics should not crash server (defer/recover in plugin calls)
- Consider plugin metrics (load time, invocation count) for observability

---

## Related Documents

- [`agent/design/plugin-architecture-backend.md`](../../design/plugin-architecture-backend.md) § Plugin Registry
- [Go plugin package](https://pkg.go.dev/plugin)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
