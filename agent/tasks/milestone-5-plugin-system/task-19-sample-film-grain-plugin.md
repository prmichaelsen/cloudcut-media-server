# Task 19: Create Sample Film-Grain Plugin

**Status**: Not Started
**Milestone**: M5 - Plugin System Foundation
**Estimated Hours**: 4-5
**Priority**: Medium

---

## Objective

Create a complete reference implementation of a Go plugin (film-grain effect) that demonstrates the full plugin lifecycle, serves as documentation, and validates the plugin system.

---

## Context

The film-grain plugin serves multiple purposes:
1. **Reference implementation** for third-party plugin developers
2. **Validation** that plugin system works end-to-end
3. **Documentation** showing best practices
4. **Testing** provides real plugin for integration tests

**Design reference**: `agent/design/plugin-architecture-backend.md` § Sample Plugin

---

## Steps

### 1. Create Plugin Directory Structure

**Action**: Create directory structure
```
plugins/film-grain/
├── plugin.yaml
├── main.go
├── plugin_test.go
├── README.md
└── Makefile
```

### 2. Write Plugin Manifest

**plugin.yaml**:

```yaml
name: film-grain
version: 1.0.0
type: go-plugin
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
        - id: color
          type: bool
          default: false
      implementation:
        type: ffmpeg-filter
```

**Action**: Create manifest

### 3. Implement Plugin Interface

**main.go**:

```go
package main

import (
    "fmt"
    "github.com/prmichaelsen/cloudcut-media-server/internal/plugin"
)

// FilmGrainEffect adds film grain noise to video
type FilmGrainEffect struct{}

// ID returns the unique plugin identifier
func (e *FilmGrainEffect) ID() string {
    return "film-grain"
}

// Name returns the human-readable plugin name
func (e *FilmGrainEffect) Name() string {
    return "Film Grain"
}

// Version returns the plugin version
func (e *FilmGrainEffect) Version() string {
    return "1.0.0"
}

// Type returns the plugin type
func (e *FilmGrainEffect) Type() plugin.PluginType {
    return plugin.PluginTypeGoPlugin
}

// BuildFFmpegFilter constructs FFmpeg filter string from parameters
func (e *FilmGrainEffect) BuildFFmpegFilter(params map[string]interface{}) (string, error) {
    // Extract intensity parameter
    intensity, ok := params["intensity"].(float64)
    if !ok {
        intensity = 0.5 // default
    }

    // Validate range
    if intensity < 0.0 || intensity > 1.0 {
        return "", fmt.Errorf("intensity must be between 0.0 and 1.0, got %.2f", intensity)
    }

    // Extract color parameter
    color, ok := params["color"].(bool)
    if !ok {
        color = false // default
    }

    // Build FFmpeg noise filter
    // noise filter: noise=alls=<strength>:allf=<flags>
    // alls: strength (0-100)
    // allf: t (temporal), p (pattern), u (uniform)
    strength := int(intensity * 100)

    filterFlags := "t+p" // temporal + pattern
    if color {
        filterFlags += "+c" // add color noise
    }

    return fmt.Sprintf("noise=alls=%d:allf=%s", strength, filterFlags), nil
}

// GetParameters returns plugin parameter definitions
func (e *FilmGrainEffect) GetParameters() []plugin.EffectParameter {
    return []plugin.EffectParameter{
        {
            ID:      "intensity",
            Type:    "float",
            Default: 0.5,
            Min:     0.0,
            Max:     1.0,
        },
        {
            ID:      "color",
            Type:    "bool",
            Default: false,
        },
    }
}

// Exported symbol for plugin loader
var Effect plugin.VideoEffectPlugin = &FilmGrainEffect{}
```

**Action**: Implement plugin in `main.go`

### 4. Write Plugin Tests

**plugin_test.go**:

```go
package main

import (
    "testing"
)

func TestFilmGrainEffect_BuildFFmpegFilter(t *testing.T) {
    effect := &FilmGrainEffect{}

    tests := []struct {
        name     string
        params   map[string]interface{}
        expected string
        wantErr  bool
    }{
        {
            name:     "default parameters",
            params:   map[string]interface{}{},
            expected: "noise=alls=50:allf=t+p",
            wantErr:  false,
        },
        {
            name: "high intensity",
            params: map[string]interface{}{
                "intensity": 1.0,
            },
            expected: "noise=alls=100:allf=t+p",
            wantErr:  false,
        },
        {
            name: "with color",
            params: map[string]interface{}{
                "intensity": 0.5,
                "color":     true,
            },
            expected: "noise=alls=50:allf=t+p+c",
            wantErr:  false,
        },
        {
            name: "invalid intensity",
            params: map[string]interface{}{
                "intensity": 1.5,
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := effect.BuildFFmpegFilter(tt.params)
            if (err != nil) != tt.wantErr {
                t.Errorf("BuildFFmpegFilter() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !tt.wantErr && result != tt.expected {
                t.Errorf("BuildFFmpegFilter() = %v, want %v", result, tt.expected)
            }
        })
    }
}

func TestFilmGrainEffect_GetParameters(t *testing.T) {
    effect := &FilmGrainEffect{}
    params := effect.GetParameters()

    if len(params) != 2 {
        t.Errorf("expected 2 parameters, got %d", len(params))
    }

    // Verify intensity parameter
    intensityParam := params[0]
    if intensityParam.ID != "intensity" {
        t.Errorf("expected first param ID 'intensity', got '%s'", intensityParam.ID)
    }
    if intensityParam.Type != "float" {
        t.Errorf("expected type 'float', got '%s'", intensityParam.Type)
    }
}
```

**Action**: Implement tests

### 5. Create Build Configuration

**Makefile**:

```makefile
.PHONY: build test clean

build:
	go build -buildmode=plugin -o plugin.so

test:
	go test -v

clean:
	rm -f plugin.so

install: build
	mkdir -p ../../plugins/film-grain
	cp plugin.so ../../plugins/film-grain/
	cp plugin.yaml ../../plugins/film-grain/
```

**Action**: Create Makefile

### 6. Write Plugin Documentation

**README.md**:

```markdown
# Film Grain Plugin

Adds film grain noise effect to video clips.

## Parameters

- **intensity** (float, 0.0-1.0, default: 0.5)
  - Controls the strength of the grain effect
  - 0.0 = no grain, 1.0 = maximum grain

- **color** (bool, default: false)
  - Whether to add color noise or monochrome
  - true = color grain, false = monochrome grain

## FFmpeg Implementation

Uses FFmpeg's `noise` filter with temporal and pattern flags.

Example output:
```
noise=alls=50:allf=t+p
```

## Building

```bash
make build
```

## Testing

```bash
make test
```

## Installing

```bash
make install
```

## Usage in EDL

```json
{
  "filters": [
    {
      "type": "film-grain",
      "params": {
        "intensity": 0.7,
        "color": false
      }
    }
  ]
}
```

## Development

This plugin serves as a reference implementation for the plugin system.
See `internal/plugin/` for plugin interfaces and `plugins/README.md`
for general plugin development documentation.
```

**Action**: Write documentation

### 7. Build and Test Plugin

**Actions**:
```bash
cd plugins/film-grain
make test    # Run unit tests
make build   # Build plugin.so
make install # Copy to plugins directory
```

### 8. Create Integration Test

**Test in server**:
1. Start server with plugin loaded
2. Submit EDL with film-grain filter
3. Verify FFmpeg command includes correct noise filter

**Action**: Add integration test to server test suite

---

## Verification

- [ ] Plugin directory structure created
- [ ] plugin.yaml manifest valid
- [ ] Plugin implements all required interfaces
- [ ] BuildFFmpegFilter() generates correct filter string
- [ ] GetParameters() returns correct schema
- [ ] Unit tests passing
- [ ] Plugin builds successfully
- [ ] Plugin loads in server without errors
- [ ] EDL with film-grain filter validates
- [ ] Render job with film-grain generates correct FFmpeg command
- [ ] README documentation complete

---

## Definition of Done

- Film-grain plugin fully implemented
- Tests passing
- Documentation complete
- Plugin builds and loads successfully
- Integration test validates end-to-end workflow
- Committed to repository

---

## Dependencies

**Blocking**:
- Task 14 (Plugin registry and interfaces)
- Task 16 (Go plugin loader)

**Optional**:
- Task 17 (EDL validator for validation tests)
- Task 18 (FFmpeg builder for render tests)

---

## Notes

- Keep plugin simple and well-documented (it's a reference)
- Use FFmpeg's noise filter (simple, built-in)
- Add comments explaining plugin interface implementation
- Consider adding more complex plugin examples later (e.g., with HTTP calls for AI)

**Future enhancements**:
- Video preview generation (thumbnail with effect applied)
- Real-time parameter adjustment
- Multiple grain presets (16mm, 35mm, VHS, etc.)

---

## Related Documents

- [`agent/design/plugin-architecture-backend.md`](../../design/plugin-architecture-backend.md) § Sample Plugin
- [FFmpeg noise filter](https://ffmpeg.org/ffmpeg-filters.html#noise)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
