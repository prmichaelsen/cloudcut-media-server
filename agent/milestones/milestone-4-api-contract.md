# Milestone 4: Stable API Contract

**Status**: Not Started
**Estimated Duration**: 2 weeks
**Priority**: High

---

## Goal

Define and document the HTTP/WebSocket API contract to enable custom backend implementations and ensure API stability across releases.

---

## Overview

This milestone establishes the stable API contract that allows users to run their own custom backends compatible with the cloudcut.media client. By creating comprehensive OpenAPI specifications, WebSocket protocol documentation, and a reference TypeScript SDK, we enable enterprise customers to deploy on-premises backends while maintaining compatibility with the managed service.

**Key principle**: API-first design with semver guarantees ensures custom backends built against v1.0 continue working with future client releases.

---

## Deliverables

1. **OpenAPI 3.0 Specification**
   - Complete REST API schema (all endpoints, request/response types)
   - Schema definitions for Media, EDL, Job, Error types
   - Authentication and authorization specs
   - Generated from code or hand-written

2. **WebSocket Protocol Documentation**
   - Message type catalog (ping, pong, edl.submit, edl.ack, job.progress, job.complete, job.error, media.status)
   - Payload schemas for each message type
   - Connection lifecycle (connect, authenticate, reconnect, disconnect)
   - Error handling and retry semantics

3. **API Reference Documentation**
   - Generated HTML docs from OpenAPI spec
   - Code examples in multiple languages (curl, TypeScript, Python, Go)
   - Authentication guide
   - WebSocket connection examples

4. **TypeScript SDK (Reference Client)**
   - HTTP client with typed methods for all endpoints
   - WebSocket client with typed message handlers
   - Automatic reconnection logic
   - Published as npm package

---

## Success Criteria

- [ ] OpenAPI spec validates with `openapi-validator`
- [ ] All current server endpoints documented in OpenAPI spec
- [ ] WebSocket protocol spec covers all message types in `internal/ws/message.go`
- [ ] TypeScript SDK successfully connects to server and completes upload → render workflow
- [ ] Reference docs generated and hosted
- [ ] Custom backend example (Python FastAPI or Node.js Express) implements API and passes compatibility tests
- [ ] API versioning strategy documented (semver, deprecation policy)

---

## Context

This milestone directly implements **Phase 1** from `agent/design/plugin-architecture-backend.md`:

> **Phase 1: Stable API Contract (M3)** [Note: M3 in design doc, but M4 in execution due to progress.yaml]
>
> **Goal**: Define and document the HTTP/WebSocket API
>
> **Actions**:
> 1. Create OpenAPI specification
> 2. Document WebSocket protocol
> 3. Publish API reference documentation
> 4. Provide reference client (TypeScript SDK)

**Design rationale**:
- **OpenAPI-first approach**: Ensures API is well-defined before implementation drifts
- **Custom backend support**: Enables enterprise on-premises deployments (HIPAA, GDPR compliance)
- **API stability**: Semver contract prevents breaking changes from impacting custom backends

---

## Dependencies

**Upstream**:
- M2 (Persistent Connection & EDL Processing) must be complete
- Current server implementation provides the API to document

**Downstream**:
- M5 (Plugin System Foundation) will extend API with plugin endpoints
- Frontend team needs API spec to generate client types

---

## Tasks

1. **Task 10**: Create OpenAPI Specification (4-6 hours)
2. **Task 11**: Document WebSocket Protocol (3-4 hours)
3. **Task 12**: Generate API Reference Docs (2-3 hours)
4. **Task 13**: Build TypeScript SDK (6-8 hours)

**Total estimated**: 15-21 hours

---

## Risks & Mitigations

**Risk 1**: API changes after documentation
- **Mitigation**: Generate OpenAPI spec from code (e.g., using Go swagger annotations) to keep in sync

**Risk 2**: Custom backends implement outdated API versions
- **Mitigation**: Version API with `/api/v1` prefix, maintain backward compatibility, publish deprecation warnings

**Risk 3**: TypeScript SDK maintenance burden
- **Mitigation**: Auto-generate SDK from OpenAPI spec using openapi-generator, minimize hand-written code

---

## Related Documents

- [`agent/design/plugin-architecture-backend.md`](../design/plugin-architecture-backend.md) - Full design specification
- [`agent/design/requirements.md`](../design/requirements.md) - Architecture decisions (EDL, WebSocket protocol)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
