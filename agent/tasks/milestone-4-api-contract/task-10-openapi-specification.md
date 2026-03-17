# Task 10: Create OpenAPI Specification

**Status**: Not Started
**Milestone**: M4 - Stable API Contract
**Estimated Hours**: 4-6
**Priority**: High

---

## Objective

Create a complete OpenAPI 3.0 specification documenting all REST API endpoints, request/response schemas, and authentication mechanisms to enable custom backend implementations.

---

## Context

The server currently exposes REST endpoints for media upload, retrieval, signed URLs, and health checks. This task formalizes the API contract using OpenAPI 3.0 to enable:
- Custom backend implementations in any language
- Auto-generated client SDKs
- API versioning and evolution
- API testing and validation

**Design reference**: `agent/design/plugin-architecture-backend.md` § OpenAPI Specification

---

## Steps

### 1. Choose OpenAPI Generation Approach

**Options**:
- **Option A**: Hand-write OpenAPI YAML (full control, manual maintenance)
- **Option B**: Generate from Go code annotations (e.g., swaggo/swag)
- **Option C**: Generate from existing code with tooling (e.g., kin-openapi)

**Recommendation**: Start with hand-written YAML for clarity, consider code generation for maintenance later.

**Action**: Create `api/openapi.yaml` skeleton

### 2. Document REST Endpoints

Extract endpoint definitions from `internal/api/router.go`:

**Endpoints to document**:
- `GET /health` - Health check
- `POST /api/v1/media/upload` - Upload media file
- `GET /api/v1/media/{id}` - Get media metadata
- `GET /api/v1/media/{id}/url` - Get signed URL for source
- `GET /api/v1/media/{id}/proxy/url` - Get signed URL for proxy
- `GET /api/v1/jobs/{id}` - Get job status (future)
- `GET /api/v1/jobs/{id}/output` - Download rendered output (future)

**For each endpoint, document**:
- HTTP method and path
- Path parameters (e.g., `{id}`)
- Query parameters
- Request body schema (multipart/form-data for upload)
- Response schemas (success and error cases)
- Status codes (200, 201, 400, 404, 500)

**Action**: Add paths section to openapi.yaml

### 3. Define Schema Components

Create reusable schemas in `components/schemas`:

**Schemas needed** (infer from `pkg/models` and handlers):
- `Media` - Media metadata (id, filename, size, status, gcsPath, etc.)
- `MediaStatus` - Enum (uploading, processing, ready, error)
- `Error` - Error response (code, message)
- `SignedURL` - Signed URL response (url)
- `Job` - Render job (id, status, progress, outputPath, etc.) [future]
- `JobStatus` - Enum (queued, processing, complete, error) [future]

**Action**: Add schemas section with all types

### 4. Add Authentication Spec

**Current state**: No authentication (MVP)
**Future state**: API key or OAuth2

**Action**:
- Document authentication as optional for now
- Add security schemes section (empty or with placeholder)
- Note in description that auth will be added post-MVP

### 5. Validate OpenAPI Spec

**Action**:
- Install OpenAPI validator (e.g., `npm install -g @apidevtools/swagger-cli`)
- Run validation: `swagger-cli validate api/openapi.yaml`
- Fix any errors

### 6. Add Metadata and Info

Complete the `info` section:
- Title: "CloudCut Media Server API"
- Version: "1.0.0"
- Description: Standard API contract for custom backends
- Contact, license

**Action**: Fill in metadata

### 7. Document API Versioning Strategy

**Action**:
- Add note in description about semver and breaking changes
- Document deprecation policy
- Confirm `/api/v1` prefix for all endpoints

---

## Verification

- [ ] `api/openapi.yaml` exists and validates with `swagger-cli validate`
- [ ] All current endpoints from router.go are documented
- [ ] All request/response types have schemas
- [ ] Error responses documented (400, 404, 500)
- [ ] Health endpoint documented
- [ ] Upload endpoint with multipart/form-data documented
- [ ] Media retrieval endpoints documented
- [ ] Schemas match actual handler responses (check with curl tests)
- [ ] OpenAPI viewer (e.g., Swagger UI) can render the spec

---

## Definition of Done

- OpenAPI 3.0 spec created in `api/openapi.yaml`
- All current endpoints documented
- All schemas defined
- Spec validates successfully
- Committed to repository

---

## Dependencies

**Blocking**:
- M2 complete (REST API endpoints exist)

**Required Files**:
- `internal/api/router.go` - Endpoint definitions
- `internal/api/handlers.go` - Request/response types
- `pkg/models/media.go` - Media model

---

## Notes

- Use OpenAPI 3.0 (not 2.0/Swagger)
- Prefer YAML over JSON for readability
- Keep schemas DRY with `$ref` references
- Document both success and error cases
- Add examples for request/response bodies

---

## Related Documents

- [`agent/design/plugin-architecture-backend.md`](../../design/plugin-architecture-backend.md) § OpenAPI Specification
- [OpenAPI 3.0 Specification](https://swagger.io/specification/)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
