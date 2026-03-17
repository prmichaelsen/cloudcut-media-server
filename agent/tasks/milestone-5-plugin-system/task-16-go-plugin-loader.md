# Task 16: Build Go Plugin Loader

**Status**: Not Started
**Milestone**: M5 - Plugin System Foundation
**Estimated Hours**: 4-5
**Priority**: High

---

## Objective

Implement a Go plugin loader that dynamically loads compiled `.so` plugins using Go's `plugin` package and registers them with the plugin registry.

---

## Context

Go plugins are compiled shared libraries (`.so` on Linux, `.dylib` on macOS) that can be loaded at runtime. They must:
- Be compiled with the same Go version as the server
- Export symbols for the plugin system to discover
- Implement the plugin interfaces defined in Task 14

**Design reference**: `agent/design/plugin-architecture-backend.md` § Go Plugins

---

## Steps

### 1. Define Plugin Symbol Contract

**Plugins must export**:
- `var Effect plugin.VideoEffectPlugin` - The plugin instance

**Example plugin** (for reference):

```go
// plugins/film-grain/main.go
package main

import "github.com/prmichaelsen/cloudcut-media-server/internal/plugin"

type FilmGrainEffect struct{}

func (e *FilmGrainEffect) ID() string { return "film-grain" }
func (e *FilmGrainEffect) Name() string { return "Film Grain" }
func (e *FilmGrainEffect) Version() string { return "1.0.0" }
func (e *FilmGrainEffect) Type() plugin.PluginType { return plugin.PluginTypeGoPlugin }

func (e *FilmGrainEffect) BuildFFmpegFilter(params map[string]interface{}) (string, error) {
    intensity, _ := params["intensity"].(float64)
    return fmt.Sprintf("noise=alls=%d:allf=t", int(intensity*100)), nil
}

func (e *FilmGrainEffect) GetParameters() []plugin.EffectParameter {
    return []plugin.EffectParameter{{
        ID: "intensity",
        Type: "float",
        Default: 0.5,
        Min: 0.0,
        Max: 1.0,
    }}
}

// Exported symbol
var Effect plugin.VideoEffectPlugin = &FilmGrainEffect{}
```

**Action**: Document symbol contract

### 2. Implement Go Plugin Loader

**Loader structure**:

```go
type GoPluginLoader struct {
    pluginDir string
}

func NewGoPluginLoader(pluginDir string) *GoPluginLoader {
    return &GoPluginLoader{pluginDir: pluginDir}
}

func (l *GoPluginLoader) Load(manifest *PluginManifest) (Plugin, error) {
    // Build plugin path
    pluginPath := filepath.Join(l.pluginDir, manifest.Name, manifest.Runtime.Path)

    // Load plugin
    p, err := plugin.Open(pluginPath)
    if err != nil {
        return nil, fmt.Errorf("open plugin: %w", err)
    }

    // Lookup symbol
    symbol, err := p.Lookup(manifest.Runtime.Entrypoint)
    if err != nil {
        return nil, fmt.Errorf("lookup symbol %q: %w", manifest.Runtime.Entrypoint, err)
    }

    // Type assert to plugin interface
    pluginInstance, ok := symbol.(*VideoEffectPlugin)
    if !ok {
        return nil, fmt.Errorf("symbol %q is not VideoEffectPlugin", manifest.Runtime.Entrypoint)
    }

    return *pluginInstance, nil
}
```

**Action**: Implement in `internal/plugin/loader.go`

### 3. Add Error Handling

**Error cases**:
- Plugin file not found → clear error with path
- Plugin load failure → include OS error details
- Symbol not found → list available symbols (debug mode)
- Wrong symbol type → explain expected type
- Plugin panic during load → recover and return error

**Action**: Add comprehensive error handling with context

### 4. Add Platform Checks

**Platform-specific considerations**:
- Linux: `.so` files
- macOS: `.dylib` files (experimental)
- Windows: Not supported (Go plugins are Unix-only)

**Check at startup**:
```go
func init() {
    if runtime.GOOS == "windows" {
        log.Warn("Go plugins not supported on Windows, plugin system disabled")
    }
}
```

**Action**: Add platform checks and warnings

### 5. Integrate with Plugin Registry

**Integration point** in `registry.LoadPlugins()`:

```go
func (r *PluginRegistry) LoadPlugins(pluginDir string) error {
    loader := NewGoPluginLoader(pluginDir)

    manifests, err := r.discoverManifests(pluginDir)
    for _, manifest := range manifests {
        if manifest.Type != PluginTypeGoPlugin {
            continue // Skip non-Go plugins
        }

        plugin, err := loader.Load(manifest)
        if err != nil {
            log.Errorf("failed to load plugin %s: %v", manifest.Name, err)
            continue
        }

        r.Register(plugin)
    }
}
```

**Action**: Wire loader into registry

### 6. Add Plugin Build Script

**Script to build plugins**:

```bash
#!/bin/bash
# scripts/build-plugin.sh

PLUGIN_NAME=$1
PLUGIN_DIR="plugins/${PLUGIN_NAME}"

if [ ! -d "$PLUGIN_DIR" ]; then
    echo "Plugin directory not found: $PLUGIN_DIR"
    exit 1
fi

cd "$PLUGIN_DIR"

echo "Building plugin: $PLUGIN_NAME"
go build -buildmode=plugin -o plugin.so

echo "Plugin built: ${PLUGIN_DIR}/plugin.so"
```

**Action**: Create build script and document usage

### 7. Write Tests

**Test cases**:

```go
func TestGoPluginLoader_Load(t *testing.T) // happy path
func TestGoPluginLoader_MissingPlugin(t *testing.T)
func TestGoPluginLoader_InvalidSymbol(t *testing.T)
func TestGoPluginLoader_WrongSymbolType(t *testing.T)
```

**Test plugin fixture**:
- Create minimal test plugin in `testdata/plugins/test-effect/`
- Build as part of test setup
- Load and verify in tests

**Action**: Implement tests in `internal/plugin/loader_test.go`

### 8. Document Plugin Development

**Create `plugins/README.md`**:
- How to create a plugin
- Required interfaces to implement
- How to build plugins
- How to test plugins
- Deployment instructions

**Action**: Write plugin developer guide

---

## Verification

- [ ] `internal/plugin/loader.go` created
- [ ] GoPluginLoader loads `.so` files successfully
- [ ] Symbol lookup works for exported plugin variables
- [ ] Type assertions validate plugin interfaces
- [ ] Missing plugin returns clear error
- [ ] Invalid symbol returns clear error
- [ ] Platform checks warn on Windows
- [ ] Plugin build script works
- [ ] Tests with real plugin fixture pass
- [ ] Plugin developer guide written

---

## Definition of Done

- Go plugin loader implemented
- Error handling complete
- Integration with registry working
- Plugin build script created
- Tests passing
- Developer documentation written
- Committed to repository

---

## Dependencies

**Blocking**:
- Task 14 (Plugin registry and interfaces)
- Task 15 (Manifest types)

**Platform Requirements**:
- Linux or macOS (Go plugins not supported on Windows)
- Go version must match between server and plugins

---

## Notes

- Go plugins require exact Go version match (major.minor.patch)
- Plugins must be rebuilt when Go version changes
- Consider Docker build environment for reproducible builds
- Symbol names are case-sensitive
- Plugin panics can crash server if not caught

**Future enhancements**:
- Hot reload (watch plugin directory for changes)
- Plugin versioning (load multiple versions simultaneously)
- Plugin isolation (run in separate processes for safety)

---

## Related Documents

- [`agent/design/plugin-architecture-backend.md`](../../design/plugin-architecture-backend.md) § Go Plugins
- [Go plugin package](https://pkg.go.dev/plugin)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
