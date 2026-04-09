# GCPlane (v1.2.0 — stable since 2026-04-05)

Declarative GitOps control plane for GoClaw. Manages AI agents, providers, channels, MCP servers, cron jobs, and teams through YAML manifests.

## Core Principles
- **Deploy-Anywhere**: Single binary for local, VPS, k8s. No platform lock-in.
- **Minimal Dependencies**: Stdlib preferred. No SDKs for simple HTTP calls.
- **Self-Contained**: Config via env vars + YAML manifest only.

## Tech Stack
- Go 1.25, Cobra CLI, gorilla/websocket, gopkg.in/yaml.v3
- GoClaw API: HTTP REST + WebSocket RPC v3

## Architecture
```
cmd/              — CLI commands (validate, plan, apply, diff, export, serve, destroy, init, status, top)
internal/
  manifest/       — YAML loader, validator, composites, labels, field config
  reconciler/     — Observe→Compare→Act engine with ReconcileOpts (DryRun, Prune)
  provider/goclaw — GoClaw API client (HTTP + WS) for 14 resource types
  keyconv/        — camelCase↔snake_case key translation
  controller/     — Reconcile loop, status tracker, tenant manager
  server/         — HTTP endpoints (health, metrics, status, sync, webhook)
  source/         — Manifest sources (file with SHA256, git with clone/fetch)
  display/        — Colored terminal output (plan, diff, prune warning)
  secrets/        — ${ENV_VAR} and file:// resolution
  tui/            — Interactive terminal UI (k9s-style resource browser with vim keybindings)
    views/        — View components (table, detail, drift, trace, help, confirm)
    trace/        — slog ring buffer handler for trace capture
```

## Key Patterns
- Manifest uses camelCase (k8s convention), provider translates to snake_case for GoClaw API
- `WriteOnlyFields` in `manifest/field_config.go` — fields excluded from comparison (secrets, grants, tokens, JSONB configs)
- `stripInternal` in `provider/goclaw/helpers.go` — removes API-internal fields from observe results
- CronJob observe has additional camelCase stripping (WS RPC returns camelCase unlike HTTP)
- CronJob `deliver`, `deliverChannel`, `deliverTo`, `stateless`, `wakeHeartbeat`, `deleteAfterRun` are observable (top-level columns since GoClaw upstream promotion from payload JSONB)
- CronJob `message` and `agentKey` are write-only; `agentKey` resolved to `agentId` UUID on create/update
- Prune: `--prune` flag, deletes in reverse `DeleteOrder()`, only `created_by=gcplane`
- Composites: `CompositeDefinition` expanded during load via Go `text/template`
- Agent `contextWindow` and `maxToolIterations` are observable (manageable from manifests)
- Agent v3 promoted scalars (`emoji`, `agentDescription`, `thinkingLevel`, `maxTokens`, `selfEvolve`, `skillEvolve`, `skillNudgeInterval`) are observable
- Agent v3 promoted JSONB configs (`reasoningConfig`, `workspaceSharing`, `chatgptOauthRouting`, `shellDenyGroups`, `kgDedupConfig`) are write-only
- Provider `apiKey` is write-only (masked as "***" in API responses)
- SecureCLI `env` is write-only (encrypted env vars for CLI credential injection)
- SecureCLIGrant uses composite name `binaryName--agentKey` (e.g., `kubectl--assistant`)

## TUI Architecture
- **Tabbed Layout** (v1.2+): 3 full-screen peer tabs (State, Traces, Logs) switched via s/t/l keys
- `View` interface: `Name()`, `Primitive()`, `Activate()` — all views implement this
- `ViewRegistry`: manages tab pages with overlay support (confirm modal, help, span detail)
- `EventBus`: typed pub/sub with `QueueUpdateDraw` thread safety
- `LiveStore`: thread-safe shared state with atomic updates for real-time data
- `TraceStore`: thread-safe LLM trace store with dirty flags, WS event forwarding, list/detail refresh
- `RingHandler` (`trace/`): custom `slog.Handler` → 1000-entry ring buffer for slog API call tracing
- **Tab Components**:
  - State tab: ResourceTable (kind filtering via 0-9), ResourceDetail (Enter), DriftView (d key)
  - Traces tab: Split panel — TraceList (left, 2:3 ratio) | SpanTree (right), Tab key toggles focus, Enter on span shows SpanDetail overlay
  - Logs tab: LogsPanel (level filtering 1-4, live streaming)
- **Trace Data Flow**: WS `trace.updated` → TraceStore.NotifyTraceUpdated() → tabRefreshLoop calls RefreshList/RefreshDetail → UI update via QueueUpdateDraw
- **Span Tree**: Real LLM agent traces from GoClaw `/v1/traces` API, sorted by StartTime for deterministic ordering, color-coded by span type (agent=Mauve, llm_call=Blue, tool_call=Teal)
- API logging: HTTPClient logs request/response (method, path, status, duration) via slog
- Keybindings: Tab switch (s/t/l), edit (Ctrl+E), delete (Ctrl+D), reconcile (Ctrl+R), pause/resume (Space), clear filters (c), Traces-specific: Tab=focus toggle, Enter=select/expand, y=copy ID/info

## Testing
```bash
mise run test          # unit tests
mise run test:e2e      # reset GoClaw + test all commands
mise run reset         # wipe GoClaw + re-apply manifest
mise run serve         # continuous reconciliation
```

## Local Dev
```bash
cp .env.example .env   # fill in credentials
mise run setup         # start GoClaw docker + apply config
```

## GoClaw Compatibility
Tested against `ghcr.io/nextlevelbuilder/goclaw:full` (v3.x — v3.1.1+)
- v3 promoted 12 agent fields from `other_config` JSONB to top-level columns
- v3 deprecated `agentType: open` (defaults to `predefined`)
- v3 added Facebook Messenger and Pancake channel types
