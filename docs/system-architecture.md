# GCPlane System Architecture

## Overview

GCPlane is a GitOps-style control plane for managing GoClaw deployments. It reads declarative YAML manifests and reconciles them against the actual GoClaw state via HTTP REST and WebSocket RPC APIs.

## Architecture Diagram

```
┌──────────────────────────────────────────────────────────────┐
│                           CLI Layer                          │
│  gcplane validate | plan | apply | serve [--prune] [--repo]  │
└──────────┬───────────────────────────────────────────────────┘
           │
┌──────────▼───────────────────────────────────────────────────┐
│                 Manifest Source (File or Git)                │
│  File: SHA256 change detection                               │
│  Git: Clone/fetch from branch, SHA change detection          │
└──────────┬───────────────────────────────────────────────────┘
           │
┌──────────▼───────────────────────────────────────────────────┐
│                      Manifest Loader                         │
│  YAML parsing → validation → secret resolution               │
│  Supports: single file, directory merge, env vars, files     │
└──────────┬───────────────────────────────────────────────────┘
           │
┌──────────▼───────────────────────────────────────────────────┐
│                   Reconciler Engine                          │
│  Observe → Compare → Act                                     │
│  Dependency-ordered processing (Provider → Agent → ...)      │
│  Modes: dry-run (plan) | apply | serve (continuous)          │
│  Prune: detect and delete orphaned resources in reverse order│
└──────────┬───────────────────────────────────────────────────┘
           │
┌──────────▼───────────────────────────────────────────────────┐
│                   GoClaw Provider                            │
│  ┌──────────────────────┐  ┌───────────────────┐             │
│  │     HTTP Client      │  │   WS RPC Client   │             │
│  │   REST endpoints     │  │   v3 protocol     │             │
│  │  Observe/Create/     │  │  CronJob, Team,   │             │
│  │  Update/Delete/List  │  │  TTSConfig        │             │
│  └─────────┬────────────┘  └──────┬────────────┘             │
└────────────┼──────────────────────┼──────────────────────────┘
             │                      │
┌────────────▼──────────────────────▼──────────────────────────┐
│                    GoClaw Instance                           │
│              HTTP REST + WebSocket RPC                       │
└──────────────────────────────────────────────────────────────┘
```

## Package Structure

```
gcplane/
├── main.go                          # Entry point
├── cmd/                             # CLI commands (Cobra)
│   ├── root.go                      # Root command + global flags
│   ├── config.go                    # Connection config resolution
│   ├── plan.go                      # Dry-run reconciliation + --prune flag
│   ├── apply.go                     # Apply with confirmation + --prune flag
│   ├── validate.go                  # Schema validation
│   ├── diff.go                      # Drift detection (stub)
│   ├── export.go                    # State export (stub)
│   ├── top.go                       # Interactive TUI for monitoring
│   └── serve.go                     # Continuous reconciliation (file/git sources)
├── internal/
│   ├── manifest/                    # YAML manifest handling
│   │   ├── types.go                 # Manifest, Resource, ResourceKind
│   │   ├── loader.go                # File/directory loading + merging
│   │   ├── validate.go              # Schema validation rules
│   │   └── delete_order.go          # Reverse dependency order for prune
│   ├── reconciler/                  # Observe→Compare→Act engine
│   │   ├── types.go                 # Plan, Change, FieldDiff, ApplyResult, ReconcileOpts
│   │   ├── engine.go                # Reconciliation + prune detection + secret resolution
│   │   └── compare.go              # Deep spec comparison (skips masked fields)
│   ├── provider/goclaw/             # GoClaw API provider
│   │   ├── provider.go              # Provider struct + routing + Option pattern
│   │   ├── http_client.go           # Authenticated HTTP client + tenant header
│   │   ├── ws_client.go             # WebSocket RPC v3 client + tenant handshake
│   │   ├── helpers.go               # Shared utilities (stripInternal)
│   │   ├── agents.go                # Agent CRUD
│   │   ├── providers.go             # Provider CRUD (API key masking)
│   │   ├── channels.go              # Channel CRUD
│   │   ├── mcp_servers.go           # MCP server CRUD
│   │   ├── skills.go                # Skill observe/update (not deletable)
│   │   ├── cron_jobs.go             # Cron job CRUD (WS, deletable)
│   │   ├── teams.go                 # Team CRUD (WS, deletable)
│   │   ├── tenants.go               # Tenant CRUD (system scope only)
│   │   ├── builtin_tool_configs.go  # Per-tenant builtin tool config
│   │   ├── skill_configs.go         # Per-tenant skill enable/disable
│   │   ├── mcp_credentials.go       # Per-user MCP server credentials
│   │   ├── system_configs.go        # Per-tenant system settings
│   │   ├── secure_cli.go            # SecureCLI CRUD (HTTP, deletable)
│   │   ├── secure_cli_grants.go     # SecureCLIGrant CRUD (HTTP, deletable)
│   │   └── secure_cli_test.go       # SecureCLI + Grant tests (13 tests)
│   ├── controller/                  # Reconciliation loop + status tracking
│   │   ├── controller.go            # Main loop with interval + graceful shutdown
│   │   └── status.go                # k8s-style status conditions
│   ├── source/                      # Manifest source abstraction
│   │   ├── source.go                # Source interface
│   │   ├── file_source.go           # File watching with SHA256 detection
│   │   └── git_source.go            # Git repository with clone/fetch/checkout
│   ├── server/                      # HTTP server for serve mode
│   │   ├── server.go                # Server startup + graceful shutdown
│   │   └── handlers.go              # /healthz, /readyz, /metrics, /api/v1/*
│   ├── keyconv/                     # camelCase ↔ snake_case conversion
│   │   └── keyconv.go               # Bidirectional case translation
│   ├── secrets/                     # Secret resolution
│   │   └── resolver.go              # ${ENV}, file://, SOPS support
│   ├── display/                     # Terminal output formatting
│   │   └── plan.go                  # Terraform-style colored diff + prune warnings
│   ├── tui/                         # Interactive terminal UI (k9s-style, v1.2.0+, 5,754 LOC)
│   │   ├── app.go                   # Main orchestrator, wires layout/views, refresh loops
│   │   ├── model.go                 # Thread-safe shared state (RWMutex)
│   │   ├── keybindings.go           # Vim-style input dispatch with two-key sequences
│   │   ├── registry.go              # ViewRegistry for tab page + overlay management
│   │   ├── event_bus.go             # Typed pub/sub with QueueUpdateDraw thread safety
│   │   ├── actions.go               # Resource operations (apply, delete, edit)
│   │   ├── attach.go                # HTTP polling for remote instances
│   │   ├── trace_store.go           # Thread-safe LLM trace cache with fetch gating
│   │   ├── live_store.go            # Ring-buffered WS event stream (500 entries)
│   │   └── views/
│   │       ├── view.go              # View interface (Name, Primitive, Activate)
│   │       ├── resource_table.go    # State tab: resource list with status coloring
│   │       ├── resource_detail.go   # YAML view with syntax highlighting
│   │       ├── drift_view.go        # Field-level drift diff
│   │       ├── trace_list.go        # Traces tab: list of traces (left panel, 2:3)
│   │       ├── span_tree.go         # Traces tab: span tree (right panel, 1:3)
│   │       ├── span_detail.go       # Overlay: detailed span information
│   │       ├── logs_panel.go        # Logs tab: live log table with level filtering
│   │       ├── confirm_modal.go     # Confirmation dialog for destructive ops
│   │       └── help_view.go         # Help overlay with keybinding reference
│   └── tui/trace/                   # Trace capture (v1.2.0+)
│       ├── ring_handler.go          # Custom slog.Handler with 1000-entry ring buffer
│       ├── entry.go                 # TraceEntry structure
│       └── types.go                 # Event types (EventTraceEntry, etc.)
└── examples/
    ├── minimal.yaml                 # Minimal manifest example (camelCase)
    ├── production.yaml              # Production manifest example (camelCase)
    └── local-dev.yaml               # Full-featured example (4 providers, agents, channels, tools, crons)
```

## Version & Compatibility

### GCPlane Versions
| Version | Release | Focus |
|---------|---------|-------|
| v0.1–v0.7.0 | 2026-03-17–18 | Foundation → Interactive TUI |
| v0.7.2 | 2026-03-25 | Credentials, Tenant Isolation |
| v0.8.0 | 2026-03-24 | First-class Multi-tenant (13 kinds) |
| v1.0.0 | 2026-04-02 | Production Release (14 kinds, removed TTSConfig/Tool) |
| v1.1.0 | 2026-04-05 | Secure CLI & Grants (SecureCLI/SecureCLIGrant) |
| **v1.2.0** | **2026-04-05** | **TUI Redesign: 3-tab layout, LLM trace capture, ViewRegistry architecture** |

### GoClaw Compatibility Matrix

| GCPlane | Tested GoClaw | RPC Protocol | Key Features |
|---------|---------------|--------------|--------------|
| v1.1.0+ | 2.x | v3 | 14 kinds (+ SecureCLI/Grant), full multi-tenant, stable API |

**Tested Image**: `ghcr.io/nextlevelbuilder/goclaw:full` (v2.x)

### Dependency Versions
| Dependency | Version | Usage |
|------------|---------|-------|
| Go | 1.25+ | Language |
| gorilla/websocket | 1.5.3 | RPC v3 protocol |
| cobra | 1.10.2 | CLI framework |
| gopkg.in/yaml.v3 | 3.0.1 | Manifest parsing |
| tview | 0.42.0 | Terminal UI |
| tcell/v2 | 2.8.1 | Terminal rendering |

### RPC Protocol (v3)
- **WebSocket handshake**: `{"token": "...", "user_id": "...", "tenant_id": "..."}`
- **Tenant headers**: `X-GoClaw-Tenant-Id`, `X-GoClaw-User-Id` on HTTP requests
- **Status**: Stable, tested against GoClaw 1.2.0

## Key Design Decisions

### Dual Transport
- **HTTP REST**: Primary for Provider, Agent, Channel, MCPServer, Skill, BuiltinToolConfig, SkillConfig, MCPCredentials, SystemConfig, SecureCLI, SecureCLIGrant (support Create/Update/Delete/List)
- **WebSocket RPC v3**: CronJob, AgentTeam (no HTTP endpoints in GoClaw; support Create/Update/Delete/List)
- WS connection is lazy-initialized on first WS resource access

### Credential Structure
Channel credentials are stored in a `credentials` object (not top-level). Structure varies by channel type:
- **Telegram**: `credentials.token` — single bot token
- **Slack**: `credentials.botToken` (bot token) + `credentials.appToken` (app token)
- Other channel types follow similar nested credential patterns

This prevents credential data from appearing in manifest comparisons via write-only field exclusion.

### No Local State
GoClaw API is the single source of truth. GCPlane carries no local state (SQLite removed). Every reconciliation queries live state, ensuring accuracy and simplifying deployments.

### Natural Key Resolution
GoClaw uses UUIDs internally. GCPlane uses human-readable natural keys (`name` field). Resolution: observe (list all) → filter by `name` → extract UUID for mutations.

### Dependency Ordering
Resources processed in dependency order:
Tenant → Provider → Agent → Skill → BuiltinToolConfig → SkillConfig → SystemConfig → MCPServer → MCPCredentials → Channel → CronJob → SecureCLI → SecureCLIGrant → AgentTeam

Prune deletes in reverse order (safe cascading).

### Prune Safety
- Prune is opt-in (requires `--prune` flag or manifest `prune: true`)
- Only deletes resources marked with `gcplane.io/managed: true` (GCPlane-owned)
- Skill is excluded (GoClaw manages this; not deletable)
- SystemConfig, SkillConfig, BuiltinToolConfig, MCPCredentials, SecureCLIGrant not enumerable for prune (child resource or GoClaw design)
- Deletes happen in reverse dependency order to prevent cascade failures
- Continue-on-error per-resource; one failure doesn't block others

### Tenant Isolation
In multi-tenant mode, GCPlane enforces tenant isolation via:
- **Observation filtering**: `matchesTenant()` applied to all observe/listAll results
  - Cached tenant UUID resolution prevents redundant API calls
  - Filters by `tenant_id` field in response; trusts header-based scoping if field absent
- **Creation injection**: Tenant slug resolved to UUID, injected as `tenant_id` into POST bodies
  - GoClaw API requires `tenant_id` UUID for tenant-scoped resource creation
- **Resource filtering**: Only resources matching the provider's tenant ID are considered
  - Single-tenant mode (no tenant ID set) matches all resources

### API Key Masking
GoClaw returns `"***"` for sensitive fields. Comparator skips masked fields to avoid false-positive diffs. On update, full key from manifest is always sent.

### camelCase Manifest Convention
Manifest uses Kubernetes-style camelCase keys (e.g., `displayName`, `baseUrl`, `apiKey`). Provider implements internal keyconv package to translate camelCase ↔ snake_case for GoClaw API compatibility.

### Secret Resolution
Manifest values support `${ENV_VAR}` substitution and `file://path` references. Secrets resolved at reconciliation time, not at load time.

## Data Flow

### Plan (dry-run)
1. Load + validate manifest
2. Resolve connection config (flags > env > manifest)
3. For each resource (in dependency order):
   a. Resolve secrets in spec
   b. Observe current state from GoClaw
   c. Compare desired vs actual (skip masked fields)
   d. Record: create / update / noop
4. If `--prune` flag: detect orphaned gcplane-owned resources → record delete
5. Display colored diff

### Apply
1. Same as plan
2. Show diff, prompt for confirmation (destructive warning if deletes > 0)
3. Execute creates/updates in dependency order
4. Execute deletes in reverse dependency order (if `--prune`)
5. Display results

### Serve (continuous reconciliation)
1. Initialize source (file watch or git clone)
2. Start HTTP server on `--addr` (default `:8480`)
3. Loop with `--interval` (default 30s):
   a. Check source for changes (SHA256 for files, git fetch for repos)
   b. If changed: load + validate manifest
   c. Reconcile using plan + apply flow (with prune if enabled)
   d. Update status (Synced/Error/Drifted conditions)
   e. Export Prometheus metrics
4. Expose status endpoints (/healthz, /readyz, /metrics, /api/v1/status, /api/v1/sync, /api/v1/webhook/git)
5. Graceful shutdown on SIGINT/SIGTERM

### Top (Interactive Dashboard) — v1.2.0+
1. Load + validate manifest
2. Resolve connection config (flags > env > manifest)
3. Create TUI app with ViewRegistry:
   a. Register State (ResourceTable, ResourceDetail, DriftView)
   b. Register Traces (TraceList, SpanTree, SpanDetail overlay)
   c. Register Logs (LogsPanel)
   d. Register overlays: ConfirmModal, HelpView
   e. Create EventBus for typed pub/sub with QueueUpdateDraw
   f. Create TraceStore (LLM trace cache) + LiveStore (event ring buffer, 500 entries)
   g. Create RingHandler (1000 entries) for slog API call tracing
   h. Wire reconciler/engine logger → RingHandler → EventBus
4. Refresh loops:
   - **State tab** (10s): List resources → compute status → update model → publish plan.updated
   - **Traces tab** (3s polling on dirty flag): HTTP ListTraces → BuildSpanTree → update TraceStore → QueueUpdateDraw
   - **Logs tab** (real-time): WS event stream → LiveStore ring buffer → dirty flag → 150ms refresh loop → QueueUpdateDraw
5. Handle vim-style keybindings with global + tab-specific actions:
   - **Global** (all tabs): S/T/L (uppercase tab switch), Ctrl+C/Q (quit), Esc (back), Ctrl+R/r (refresh), ? (help), : (command), / (search)
   - **State**: j/k navigate, gg/G jump, Enter (detail), d (drift), 0-9 (kind filter), yy (copy ID), c (clear)
   - **Traces**: Tab (focus toggle), l/h (drill), Enter (expand), Space/p (pause), yy (copy), c (clear)
   - **Logs**: j/k navigate, 1-4 (level filter), Space/p (pause), yy (copy), c (clear)
6. Views receive events:
   - ResourceTable subscribes to plan.updated, refreshes on state change
   - TraceList subscribes to trace.updated, polls API every 3s
   - LogsPanel subscribes to WS event stream, displays real-time logs
7. Overlay navigation: ViewRegistry stack with push/pop, Esc dismisses any overlay
8. Graceful shutdown on Ctrl+C (close WS, cleanup tview)
