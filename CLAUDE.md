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
- Prune: `--prune` flag, deletes in reverse `DeleteOrder()`, only `created_by=gcplane`
- Composites: `CompositeDefinition` expanded during load via Go `text/template`
- Agent `contextWindow` and `maxToolIterations` are observable (manageable from manifests)
- Provider `apiKey` is write-only (masked as "***" in API responses)
- SecureCLI `env` is write-only (encrypted env vars for CLI credential injection)
- SecureCLIGrant uses composite name `binaryName--agentKey` (e.g., `kubectl--assistant`)

## TUI Architecture
- **Tabbed Layout** (v1.2+): 4 full-screen peer tabs (State, Logs, Events, Trace) switched via s/l/e/t keys
- `View` interface: `Name()`, `Primitive()`, `Activate()` — all views implement this
- `ViewRegistry`: manages tab pages with overlay support (confirm modal, help)
- `EventBus`: typed pub/sub with `QueueUpdateDraw` thread safety
- `LiveStore`: thread-safe shared state with atomic updates for real-time data
- `RingHandler` (`trace/`): custom `slog.Handler` → 1000-entry ring buffer → trace view
- **Tab Components**:
  - State tab: ResourceTable (kind filtering via 0-9), ResourceDetail (Enter), DriftView (d key)
  - Logs tab: LogsPanel (level filtering 1-4, live streaming)
  - Events tab: EventsPanel (type filtering 1-6, live streaming)
  - Trace tab: TraceView (level filtering 1-4, shows API calls and reconciliation events)
- API logging: HTTPClient logs request/response (method, path, status, duration) via slog
- Keybindings: Tab switch (s/l/e/t), edit (Ctrl+E), delete (Ctrl+D), reconcile (Ctrl+R), pause/resume (Space), clear filters (c)

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
Tested against `ghcr.io/nextlevelbuilder/goclaw:full` (v2.x)
