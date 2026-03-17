# Milestone 5: Plugin System Foundation

**Status**: Not Started
**Estimated Duration**: 3 weeks
**Priority**: Medium (Post-MVP)

---

## Goal

Implement server-side plugin system with Go plugin registry, enabling third-party developers to extend the server with custom video effects, AI features, and export formats.

---

## Overview

This milestone builds the foundation for the plugin architecture by implementing the plugin registry, manifest parser, and Go plugin loader. By integrating plugins into the rendering pipeline, we enable the server to support client plugin effects without requiring core code changes for each new feature.

**Key principle**: Plugin-agnostic core with well-defined extension points. The EDL format already supports plugin references via `filters[].type` — no schema changes required.

---

## Deliverables

1. **Plugin Registry**
   - `internal/plugin/registry.go` with discovery, loading, and resolution
   - Thread-safe plugin storage and retrieval
   - Plugin lifecycle management (load, activate, deactivate)

2. **Plugin Manifest System**
   - `plugin.yaml` schema definition
   - Manifest parser and validator
   - Support for multiple plugin types (Go plugins initially)

3. **Go Plugin Loader**
   - Dynamic `.so` loading using Go `plugin` package
   - Symbol resolution for plugin entry points
   - Error handling for missing/invalid plugins

4. **EDL Validator Integration**
   - Check plugin references in EDL filters
   - Validate plugin parameters against manifest schema
   - Graceful error messages for unknown plugins

5. **FFmpeg Builder Plugin Integration**
   - Modify `internal/render/ffmpeg.go` to check plugin registry
   - Invoke `plugin.BuildFFmpegFilter()` for plugin effects
   - Fallback to built-in filters for unknown types

6. **Sample Plugin**
   - Film-grain effect plugin as reference implementation
   - Demonstrates full plugin lifecycle
   - Includes unit tests for plugin interface

---

## Success Criteria

- [ ] Plugin registry loads plugins from `plugins/` directory
- [ ] Sample film-grain plugin builds and loads successfully
- [ ] EDL with plugin filter (`"type": "film-grain"`) validates correctly
- [ ] Render job with plugin effect generates correct FFmpeg command
- [ ] Plugin error (missing plugin, invalid params) returns user-friendly error
- [ ] Plugin crash/panic doesn't bring down server
- [ ] Unit tests cover plugin registry, loader, and manifest validation
- [ ] Documentation written for plugin development (README in `plugins/`)

---

## Context

This milestone directly implements **Phase 2** from `agent/design/plugin-architecture-backend.md`:

> **Phase 2: Plugin Registry (Post-MVP)**
>
> **Goal**: Support server-side plugins on managed backend
>
> **Actions**:
> 1. Implement `internal/plugin/registry.go`
> 2. Add plugin discovery and loading
> 3. Extend EDL validator to check plugin references
> 4. Update FFmpeg builder to invoke plugins
>
> **Recommended Plugin Type for MVP**: **Go plugins** (native performance, simpler than containers)

**Design rationale**:
- **Go plugins first**: Native performance, same language as core, simpler than WASM/containers
- **Defer sandboxing**: Security via code review and trusted plugins initially, add WASM/container isolation later
- **Seamless EDL integration**: Existing `filters[].type` field accommodates plugins with zero schema changes

**Extension points implemented**:
- `effects.video` - Custom video effects (FFmpeg filters, shaders)
- `effects.audio` - Audio processing (FFmpeg audio filters)
- `export.formats` - Custom export codecs (future expansion)

---

## Dependencies

**Upstream**:
- M2 (Persistent Connection & EDL Processing) must be complete (EDL validator, FFmpeg builder exist)
- M4 (Stable API Contract) recommended but not blocking (API doesn't change)

**Downstream**:
- M6 (Plugin Marketplace) depends on plugin system being stable
- Client plugins need server plugin support to render custom effects

---

## Tasks

1. **Task 14**: Implement Plugin Registry (4-6 hours)
2. **Task 15**: Create Plugin Manifest Parser (3-4 hours)
3. **Task 16**: Build Go Plugin Loader (4-5 hours)
4. **Task 17**: Integrate Plugins into EDL Validator (2-3 hours)
5. **Task 18**: Update FFmpeg Builder for Plugin Invocation (3-4 hours)
6. **Task 19**: Create Sample Film-Grain Plugin (4-5 hours)

**Total estimated**: 20-27 hours

---

## Risks & Mitigations

**Risk 1**: Go plugins require matching Go version
- **Mitigation**: Document Go version requirement, provide Docker build environment for consistent builds

**Risk 2**: Plugin crash brings down server
- **Mitigation**: Defer panics in plugin calls, return errors gracefully, log plugin failures

**Risk 3**: Plugin API breaking changes affect third-party plugins
- **Mitigation**: Version plugin API, maintain backward compatibility, publish deprecation warnings

**Risk 4**: Security vulnerabilities in malicious plugins
- **Mitigation**: Initially support trusted/internal plugins only, defer public marketplace to M6

---

## Out of Scope (Deferred to Future Milestones)

- **WASM plugins**: Portable, sandboxed plugins (P8)
- **Container plugins**: Docker-based microservice plugins for AI features (P9)
- **Script plugins**: Lua/JavaScript for rapid development (P9)
- **Plugin marketplace**: Discovery, distribution, monetization (M6, P10)
- **Plugin sandboxing**: Security isolation, resource limits (P11)
- **Hot reload**: Update plugins without server restart (P12)

---

## Related Documents

- [`agent/design/plugin-architecture-backend.md`](../design/plugin-architecture-backend.md) - Full design specification
- [`agent/design/requirements.md`](../design/requirements.md) - Architecture decisions
- [`internal/render/ffmpeg.go`](../../internal/render/ffmpeg.go) - FFmpeg builder to be extended

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
