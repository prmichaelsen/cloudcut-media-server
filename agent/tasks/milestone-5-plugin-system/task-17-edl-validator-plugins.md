# Task 17: Integrate Plugins into EDL Validator

**Status**: Not Started
**Milestone**: M5 - Plugin System Foundation
**Estimated Hours**: 2-3
**Priority**: Medium

---

## Objective

Extend the EDL validator to check plugin references in filters, validate plugin parameters against manifest schemas, and provide clear error messages for unknown plugins.

---

## Context

Currently, the EDL validator checks built-in filter types but doesn't validate plugin references. With the plugin system in place, we need to:
- Check if `filter.type` is a registered plugin
- Validate plugin parameters against plugin manifest schema
- Provide helpful errors when plugins are missing or params are invalid

**Design reference**: `agent/design/plugin-architecture-backend.md` § EDL Validator Integration

---

## Steps

### 1. Add Plugin Registry to Validator

**Current validator signature**:
```go
func Validate(edl *EDL, mediaExists MediaExistsFn) ValidationErrors
```

**Updated signature**:
```go
func Validate(edl *EDL, mediaExists MediaExistsFn, pluginRegistry *plugin.PluginRegistry) ValidationErrors
```

**Action**: Update validator signature in `internal/edl/validate.go`

### 2. Extend Filter Validation

**Current filter validation**:
```go
func validateClip(clip Clip, field string, mediaExists MediaExistsFn) ValidationErrors {
    // ... existing validation ...

    // Apply clip filters
    for _, filter := range clip.Filters {
        // Currently just ignores filter validation
    }
}
```

**Enhanced filter validation**:
```go
func validateClip(clip Clip, field string, mediaExists MediaExistsFn, pluginRegistry *plugin.PluginRegistry) ValidationErrors {
    var errs ValidationErrors

    // ... existing validation ...

    // Validate filters
    for i, filter := range clip.Filters {
        filterField := fmt.Sprintf("%s.filters[%d]", field, i)
        errs = append(errs, validateFilter(filter, filterField, pluginRegistry)...)
    }

    return errs
}
```

**Action**: Update `validateClip()` to validate filters

### 3. Implement Filter Validation

**New validation function**:

```go
func validateFilter(filter Filter, field string, pluginRegistry *plugin.PluginRegistry) ValidationErrors {
    var errs ValidationErrors

    // Check if filter type is built-in
    if isBuiltInFilter(filter.Type) {
        // Validate built-in filter params (optional, simple check)
        return errs
    }

    // Check if filter type is a registered plugin
    if pluginRegistry != nil {
        effect, err := pluginRegistry.GetEffect(filter.Type)
        if err != nil {
            errs = append(errs, ValidationError{
                Field:   field + ".type",
                Message: fmt.Sprintf("unknown filter type %q (not built-in or registered plugin)", filter.Type),
            })
            return errs
        }

        // Validate params against plugin schema
        errs = append(errs, validatePluginParams(filter.Params, effect.GetParameters(), field)...)
    }

    return errs
}
```

**Action**: Implement `validateFilter()` in `internal/edl/validate.go`

### 4. Implement Plugin Parameter Validation

**Parameter validation**:

```go
func validatePluginParams(params map[string]interface{}, schema []plugin.EffectParameter, field string) ValidationErrors {
    var errs ValidationErrors

    // Check required parameters
    for _, paramDef := range schema {
        value, ok := params[paramDef.ID]
        if !ok {
            // Use default if available
            if paramDef.Default != nil {
                continue
            }
            errs = append(errs, ValidationError{
                Field:   field + ".params." + paramDef.ID,
                Message: "required parameter missing",
            })
            continue
        }

        // Validate type
        if !validateParamType(value, paramDef.Type) {
            errs = append(errs, ValidationError{
                Field:   field + ".params." + paramDef.ID,
                Message: fmt.Sprintf("expected type %s, got %T", paramDef.Type, value),
            })
            continue
        }

        // Validate range for numeric types
        if paramDef.Min != nil || paramDef.Max != nil {
            if err := validateParamRange(value, paramDef.Min, paramDef.Max); err != nil {
                errs = append(errs, ValidationError{
                    Field:   field + ".params." + paramDef.ID,
                    Message: err.Error(),
                })
            }
        }
    }

    // Check for unknown parameters (optional, could warn)
    for paramID := range params {
        if !hasParameter(paramID, schema) {
            // Optional: warn about unknown params
        }
    }

    return errs
}

func validateParamType(value interface{}, expectedType string) bool {
    switch expectedType {
    case "float":
        _, ok := value.(float64)
        return ok
    case "int":
        _, ok := value.(float64) // JSON numbers are float64
        return ok
    case "string":
        _, ok := value.(string)
        return ok
    case "bool":
        _, ok := value.(bool)
        return ok
    default:
        return false
    }
}

func validateParamRange(value, min, max interface{}) error {
    // Type-specific range checks
    switch v := value.(type) {
    case float64:
        if min != nil && v < min.(float64) {
            return fmt.Errorf("value %v below minimum %v", v, min)
        }
        if max != nil && v > max.(float64) {
            return fmt.Errorf("value %v above maximum %v", v, max)
        }
    }
    return nil
}
```

**Action**: Implement parameter validation helpers

### 5. Add Built-in Filter Check

**Built-in filters** (from `internal/render/ffmpeg.go`):
- brightness, contrast, saturation, crop, text

**Helper**:
```go
func isBuiltInFilter(filterType string) bool {
    builtIn := map[string]bool{
        "brightness": true,
        "contrast":   true,
        "saturation": true,
        "crop":       true,
        "text":       true,
    }
    return builtIn[filterType]
}
```

**Action**: Implement built-in filter check

### 6. Update Parse Function

**Current**:
```go
func Parse(data []byte, mediaExists MediaExistsFn) (*EDL, ValidationErrors)
```

**Updated**:
```go
func Parse(data []byte, mediaExists MediaExistsFn, pluginRegistry *plugin.PluginRegistry) (*EDL, ValidationErrors)
```

**Action**: Update `Parse()` signature and callers

### 7. Update WebSocket Handler

**In `cmd/server/main.go`**:

```go
func handleEDLSubmit(session *ws.Session, msg *ws.Message, handlers *api.Handlers, renderer *render.Renderer, pluginRegistry *plugin.PluginRegistry) {
    mediaExists := func(mediaID string) bool {
        _, ok := handlers.GetMedia(mediaID)
        return ok
    }

    parsedEDL, errs := edl.Parse(msg.Payload, mediaExists, pluginRegistry)
    // ... rest of handler ...
}
```

**Action**: Pass plugin registry to EDL parser

### 8. Write Tests

**Test cases**:

```go
func TestValidateFilter_BuiltIn(t *testing.T)
func TestValidateFilter_Plugin(t *testing.T)
func TestValidateFilter_UnknownPlugin(t *testing.T)
func TestValidatePluginParams_Valid(t *testing.T)
func TestValidatePluginParams_MissingRequired(t *testing.T)
func TestValidatePluginParams_WrongType(t *testing.T)
func TestValidatePluginParams_OutOfRange(t *testing.T)
```

**Test setup**:
- Create mock plugin registry with test effects
- Create EDLs with valid/invalid plugin filters
- Verify validation errors

**Action**: Implement tests in `internal/edl/validate_test.go`

---

## Verification

- [ ] EDL validator accepts plugin registry parameter
- [ ] Filters validated against plugin registry
- [ ] Built-in filters still work
- [ ] Unknown plugin returns clear error
- [ ] Missing required params caught
- [ ] Wrong parameter types caught
- [ ] Out-of-range values caught
- [ ] All tests passing
- [ ] WebSocket handler passes plugin registry

---

## Definition of Done

- EDL validator integrated with plugin registry
- Filter validation complete (built-in + plugins)
- Parameter validation against manifest schemas
- Tests passing
- WebSocket handler updated
- Committed to repository

---

## Dependencies

**Blocking**:
- Task 14 (Plugin registry)
- Task 15 (Manifest with parameter schemas)
- M2 Task 5 (EDL validator exists)

**Files to modify**:
- `internal/edl/validate.go`
- `internal/edl/parser.go`
- `cmd/server/main.go`

---

## Notes

- Validation should be strict but helpful (clear error messages)
- Consider providing suggestions for unknown filter types
- Plugin registry can be nil (no plugins loaded) → treat all as built-in
- Default parameter values should be applied before validation

---

## Related Documents

- [`agent/design/plugin-architecture-backend.md`](../../design/plugin-architecture-backend.md) § EDL Validator Integration

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
