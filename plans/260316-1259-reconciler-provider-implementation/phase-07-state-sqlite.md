---
phase: 7
priority: medium
effort: S
status: pending
---

# Phase 7: SQLite State Store

## Overview

Implement the state.Store interface with SQLite for CLI mode. Tracks external IDs and spec hashes to detect local manifest changes.

## Related Code Files

- **Create:** `internal/state/sqlite.go` — SQLite implementation
- **Modify:** `internal/state/store.go` — add constructor if needed

## Implementation Steps

1. Use `modernc.org/sqlite` (pure Go, no CGO) or `mattn/go-sqlite3`
   - Prefer `modernc.org/sqlite` for cross-compilation ease

2. Create table on init:
   ```sql
   CREATE TABLE IF NOT EXISTS resource_state (
       kind TEXT NOT NULL,
       key TEXT NOT NULL,
       external_id TEXT,
       spec_hash TEXT NOT NULL,
       synced BOOLEAN NOT NULL DEFAULT 0,
       last_sync TEXT NOT NULL,
       error TEXT,
       PRIMARY KEY (kind, key)
   );
   ```

3. Implement Store interface methods:
   - `Get(kind, key)` → SELECT by PK
   - `Put(state)` → INSERT OR REPLACE
   - `List()` → SELECT all
   - `Delete(kind, key)` → DELETE by PK
   - `Close()` → close DB

4. Default path: `.gcplane/state.db` relative to manifest location

5. Spec hash: SHA256 of JSON-serialized spec (after secret resolution)

## Todo

- [ ] Add SQLite dependency
- [ ] Create sqlite.go with table init
- [ ] Implement Get/Put/List/Delete/Close
- [ ] Wire into reconciler (optional — engine can work without state)
- [ ] Compile check

## Success Criteria

- State persists across CLI runs
- Can detect "manifest changed since last apply" via spec hash
- Clean DB file management
