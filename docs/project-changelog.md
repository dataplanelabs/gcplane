# GCPlane Project Changelog

All notable changes to GCPlane are documented here.

## [v1.2.0] — 2026-04-05

### Added
- **TUI LLM Agent Trace Monitoring**: New 3-tab Traces tab replacing internal Events and Trace tabs. Features real-time LLM agent execution traces from GoClaw `/v1/traces` API with split-panel drill-down.
  - TraceStore: Thread-safe trace data store with WS event forwarding and automatic refresh on `trace.updated` events.
  - TraceList: Table component showing traces with Name, Status, Duration, Token counts, Span count, and Time columns.
  - SpanTree: TreeView component displaying span hierarchy with expand/collapse, color-coding by span type (agent/llm_call/tool_call), and detailed metrics.
  - SpanDetail: Overlay view showing comprehensive span information including tokens, cache metrics, thinking tokens, and input/output previews.
  - TracesPanel: Split-panel composite view combining TraceList (left) and SpanTree (right) with Tab key focus toggle.
  - Trace API methods in GoClaw provider: `ListTraces()`, `GetTrace()` for HTTP access to `/v1/traces` and `/v1/traces/{id}`.
  - Deterministic span tree ordering by StartTime for consistent visualization.

### Changed
- **Tab Layout**: Replaced 4-tab layout (State/Logs/Events/Trace) with 3-tab layout (State/Traces/Logs).
- **Tab Keybindings**: Tab switch now via `s` (State), `t` (Traces), `l` (Logs). Old `e` (Events) and implicit 4th tab removed.
- **CLAUDE.md Architecture**: Updated TUI Architecture section to document 3-tab layout, TraceStore, and real LLM trace sourcing.
- **Logger Configuration**: Fixed TUI logger to use `io.Discard` in TUI mode, preventing log bleed-through to terminal UI.

### Removed
- **Old Views**: Deleted `internal/tui/views/events_panel.go` and `internal/tui/views/trace_view.go` (slog-based internal trace).
- **Config.TraceHandler**: Removed from TUI Config struct (trace.RingHandler still available for slog integration in cmd/).
- **Event Tab**: Removed `TabEvents` enum value and event filtering UI. Events data still buffered in LiveStore for potential future use.
- **Event Keybindings**: Removed number key handlers for event type filtering (1-6 keys).

### Fixed
- **Thread Safety**: TraceStore uses mutex protection with dirty flags for safe concurrent updates from WS handler and UI goroutines.
- **Selection Persistence**: Selected trace persists across list refreshes; trace ID guard prevents stale detail overwrites during rapid selection changes.
- **Focus Management**: Tab key on Traces tab correctly toggles focus between list and tree panels via InputCapture handler.

### Technical Notes
- Traces data sourced from GoClaw `/v1/traces` API (requires GoClaw v2.x with traces endpoint).
- Provider methods optional via `TraceFetcher` interface; graceful fallback if provider doesn't support traces.
- Span tree roots auto-expanded; children collapsed by default. Lazy detail fetch on user selection.
- Cache read/creation tokens and thinking tokens displayed per span in detail overlay.

### Backwards Compatibility
- Old TUI `:trace` command removed; use new `:traces` command for Traces tab.
- Old TUI `:events` command removed; event data still available in LiveStore for programmatic access.
- Internal slog trace capture (`trace.RingHandler`) unchanged; still used by cmd/ for API call logging.

---

## [v1.1.5] — 2026-04-04

### Fixed
- `contextFiles` field now synced from manifest to GoClaw agent_context_files during reconciliation.

---

## [v1.1.4] — 2026-04-02

### Added
- SHA-256 hash tracking for write-only fields to detect meaningful changes despite API masking.

---

## [v1.2.0] — 2026-04-05 (Earlier Releases)

### Previous Versions
- v1.1.5, v1.1.4, v1.1.3, ... (see git log for pre-v1.2.0 history)
