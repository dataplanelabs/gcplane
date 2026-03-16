---
phase: 1
priority: critical
effort: S
status: pending
---

# Phase 1: HTTP Client + Auth

## Overview

Create a reusable HTTP client for GoClaw REST API with auth, error handling, and JSON helpers.

## Related Code Files

- **Modify:** `internal/provider/goclaw/provider.go` — add client field
- **Create:** `internal/provider/goclaw/http_client.go` — HTTP client wrapper

## Key Insights

- GoClaw auth: `Authorization: Bearer <token>` header
- Response format varies: `{"agents":[...]}`, `{"ok":"true"}`, `{"error":"msg"}`
- Provider/channel lookup by name requires list+filter (no name-based GET)
- API keys masked as "***" in responses

## Implementation Steps

1. Create `http_client.go` with `HTTPClient` struct:
   ```go
   type HTTPClient struct {
       baseURL    string
       token      string
       httpClient *http.Client
   }
   ```

2. Implement core methods:
   - `Get(ctx, path) ([]byte, error)` — GET with auth headers
   - `Post(ctx, path, body) ([]byte, error)` — POST JSON
   - `Put(ctx, path, body) ([]byte, error)` — PUT JSON
   - `Patch(ctx, path, body) ([]byte, error)` — PATCH JSON
   - `Delete(ctx, path) error` — DELETE

3. All methods:
   - Set `Authorization: Bearer <token>`
   - Set `Content-Type: application/json`
   - Check HTTP status codes (400→ErrInvalidRequest, 401→ErrUnauthorized, 404→ErrNotFound, etc.)
   - Return parsed body bytes for caller to unmarshal

4. Add `ErrNotFound` sentinel for observe "doesn't exist" case

5. Update `Provider` struct to use `HTTPClient` instead of raw `*http.Client`

## Todo

- [ ] Create `http_client.go` with HTTPClient struct
- [ ] Implement Get/Post/Put/Patch/Delete methods
- [ ] Error handling with sentinel errors
- [ ] Update provider.go to use HTTPClient
- [ ] Compile check

## Success Criteria

- `HTTPClient` can make authenticated requests to GoClaw
- Proper error classification (not found vs auth vs server error)
- Clean separation from provider logic
