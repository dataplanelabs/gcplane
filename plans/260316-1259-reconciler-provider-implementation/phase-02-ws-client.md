---
phase: 2
priority: critical
effort: M
status: pending
---

# Phase 2: WebSocket RPC Client

## Overview

Create a WS client for GoClaw's v3 RPC protocol. Needed for cron, teams, TTS resources.

## Related Code Files

- **Create:** `internal/provider/goclaw/ws_client.go` — WebSocket RPC client

## Key Insights

- GoClaw WS protocol v3: frame types `req`/`res`/`event`
- First request MUST be `connect` method
- RequestFrame: `{ type: "req", id: "string", method: "string", params: {...} }`
- ResponseFrame: `{ type: "res", id: "string", ok: bool, payload?: {...}, error?: {...} }`
- Auth: pass token in connect params or query string
- GoClaw endpoint typically: `ws://host:port/ws`

## Implementation Steps

1. Create `ws_client.go` with `WSClient` struct:
   ```go
   type WSClient struct {
       endpoint string
       token    string
       conn     *websocket.Conn
       mu       sync.Mutex
       nextID   int64
   }
   ```

2. Use `gorilla/websocket` (same as GoClaw)

3. Implement `Connect(ctx) error`:
   - Dial `ws://<endpoint>/ws`
   - Send connect frame: `{ type: "req", id: "1", method: "connect", params: { token: "<token>" } }`
   - Wait for OK response
   - Return error if rejected

4. Implement `Call(ctx, method, params) (json.RawMessage, error)`:
   - Atomic increment `nextID`
   - Send request frame
   - Read response frames until matching ID
   - Return payload or error

5. Implement `Close() error` — clean disconnect

6. Keep it synchronous (one call at a time with mutex) — gcplane doesn't need concurrent WS

## Todo

- [ ] Add gorilla/websocket dependency
- [ ] Create ws_client.go with WSClient struct
- [ ] Implement Connect with connect handshake
- [ ] Implement Call for request/response
- [ ] Implement Close
- [ ] Compile check

## Success Criteria

- Can connect to GoClaw WS and complete handshake
- Can call RPC methods and get responses
- Clean disconnect on close
