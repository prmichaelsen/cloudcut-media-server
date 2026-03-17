# Task 18: Update FFmpeg Builder for Plugin Invocation

**Status**: Not Started
**Milestone**: M5 - Plugin System Foundation
**Estimated Hours**: 3-4
**Priority**: Medium

---

## Objective

Modify the FFmpeg command builder to check the plugin registry and invoke plugin-provided filter builders when processing EDL filters.

---

## Context

Currently, the FFmpeg builder uses a simple `buildFilterString()` function for built-in filters. With plugins, we need to:
1. Check if filter type is a registered plugin
2. If plugin, invoke `plugin.BuildFFmpegFilter(params)`
3. If built-in, use existing `buildFilterString()`
4. If neither, validation should have caught it (defensive check)

**Design reference**: `agent/design/plugin-architecture-backend.md` § FFmpeg Builder Plugin Integration

---

## Steps

### 1. Add Plugin Registry to Renderer

**Current renderer**:
```go
type Renderer struct {
    gcs     GCSClient
    ffmpeg  FFmpegClient
    storage JobStorage
}
```

**Updated**:
```go
type Renderer struct {
    gcs           GCSClient
    ffmpeg        FFmpegClient
    storage       JobStorage
    pluginRegistry *plugin.PluginRegistry
}

func NewRenderer(gcs GCSClient, ffmpeg FFmpegClient, storage JobStorage, pluginRegistry *plugin.PluginRegistry) *Renderer {
    return &Renderer{
        gcs:           gcs,
        ffmpeg:        ffmpeg,
        storage:       storage,
        pluginRegistry: pluginRegistry,
    }
}
```

**Action**: Update renderer constructor in `internal/render/job.go`

### 2. Pass Plugin Registry to FFmpeg Renderer

**Current**:
```go
type FFmpegRenderer struct {
    ffmpegPath string
}
```

**Updated**:
```go
type FFmpegRenderer struct {
    ffmpegPath     string
    pluginRegistry *plugin.PluginRegistry
}

func NewFFmpegRenderer(ffmpegPath string, pluginRegistry *plugin.PluginRegistry) *FFmpegRenderer {
    return &FFmpegRenderer{
        ffmpegPath:     ffmpegPath,
        pluginRegistry: pluginRegistry,
    }
}
```

**Action**: Update FFmpegRenderer in `internal/render/ffmpeg.go`

### 3. Update Filter Building Logic

**Current** (from M2):
```go
// Apply clip filters
for _, clipFilter := range clip.Filters {
    filterStr := buildFilterString(clipFilter)
    if filterStr != "" {
        filter += "," + filterStr
    }
}
```

**Updated**:
```go
// Apply clip filters
for _, clipFilter := range clip.Filters {
    filterStr, err := f.buildFilterWithPlugins(clipFilter)
    if err != nil {
        return "", fmt.Errorf("build filter %q: %w", clipFilter.Type, err)
    }
    if filterStr != "" {
        filter += "," + filterStr
    }
}
```

**Action**: Update filter building in `buildVideoFilter()` method

### 4. Implement Plugin-Aware Filter Builder

**New method**:

```go
func (f *FFmpegRenderer) buildFilterWithPlugins(filter edl.Filter) (string, error) {
    // Check if plugin
    if f.pluginRegistry != nil {
        effect, err := f.pluginRegistry.GetEffect(filter.Type)
        if err == nil {
            // Plugin found, invoke it
            filterStr, err := effect.BuildFFmpegFilter(filter.Params)
            if err != nil {
                return "", fmt.Errorf("plugin %q failed: %w", filter.Type, err)
            }
            return filterStr, nil
        }
    }

    // Fallback to built-in
    filterStr := buildFilterString(filter)
    if filterStr == "" {
        return "", fmt.Errorf("unknown filter type: %s", filter.Type)
    }
    return filterStr, nil
}
```

**Action**: Implement method in `internal/render/ffmpeg.go`

### 5. Add Error Handling

**Error cases**:
- Plugin not found (should have been caught by validator, defensive check)
- Plugin `BuildFFmpegFilter()` returns error
- Plugin returns empty string (invalid)
- Plugin panics (recover and return error)

**With panic recovery**:
```go
func (f *FFmpegRenderer) buildFilterWithPlugins(filter edl.Filter) (filterStr string, err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("plugin %q panicked: %v", filter.Type, r)
        }
    }()

    // ... plugin invocation ...
}
```

**Action**: Add comprehensive error handling

### 6. Update Main Server Initialization

**In `cmd/server/main.go`**:

```go
// Setup renderer with plugin registry
pluginRegistry := plugin.NewPluginRegistry()
if err := pluginRegistry.LoadPlugins("plugins/"); err != nil {
    log.Warnf("plugin loading failed: %v", err)
}

ffmpegRenderer := render.NewFFmpegRenderer(cfg.FFmpegPath, pluginRegistry)
jobStorage := render.NewMemoryJobStorage()
renderer := render.NewRenderer(nil, ffmpegRenderer, jobStorage, pluginRegistry)
```

**Action**: Wire plugin registry through initialization chain

### 7. Add Logging

**Log plugin invocations**:
- `log.Debug("using plugin %s for filter %s", pluginID, filterType)`
- `log.Error("plugin %s failed: %v", pluginID, err)`

**Action**: Add structured logging

### 8. Write Tests

**Test cases**:

```go
func TestBuildFilterWithPlugins_BuiltIn(t *testing.T)
func TestBuildFilterWithPlugins_Plugin(t *testing.T)
func TestBuildFilterWithPlugins_PluginError(t *testing.T)
func TestBuildFilterWithPlugins_PluginPanic(t *testing.T)
func TestBuildFilterWithPlugins_UnknownFilter(t *testing.T)
func TestFFmpegRenderer_WithPluginRegistry(t *testing.T) // integration test
```

**Test setup**:
- Create mock plugin registry with test effects
- Create test EDLs with plugin filters
- Verify correct FFmpeg commands generated

**Action**: Implement tests in `internal/render/ffmpeg_test.go`

### 9. Integration Test

**End-to-end test**:
1. Load plugin registry with sample plugin
2. Submit EDL with plugin filter
3. Build FFmpeg command
4. Verify plugin filter appears in filter_complex

**Action**: Add integration test

---

## Verification

- [ ] FFmpegRenderer accepts plugin registry
- [ ] Plugin filters invoke `BuildFFmpegFilter()`
- [ ] Built-in filters still work
- [ ] Plugin errors handled gracefully
- [ ] Plugin panics recovered
- [ ] Unknown filters return error
- [ ] Logging shows plugin invocations
- [ ] All tests passing
- [ ] Main server initialization wires registry correctly

---

## Definition of Done

- FFmpeg builder integrated with plugin registry
- Plugin filter invocation working
- Error handling complete
- Tests passing
- Server initialization updated
- Committed to repository

---

## Dependencies

**Blocking**:
- Task 14 (Plugin registry)
- Task 16 (Go plugin loader)
- M2 Task 6 (FFmpeg builder exists)

**Files to modify**:
- `internal/render/ffmpeg.go`
- `internal/render/job.go`
- `cmd/server/main.go`

---

## Notes

- Plugin should never crash server (use recover)
- Consider caching plugin lookups if performance becomes issue
- Log plugin invocations for debugging render failures
- Plugin errors should propagate to client via WebSocket job.error

---

## Related Documents

- [`agent/design/plugin-architecture-backend.md`](../../design/plugin-architecture-backend.md) § FFmpeg Builder Plugin Integration

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
