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
│   │   ├── provider.go              # Provider struct + routing
│   │   ├── http_client.go           # Authenticated HTTP client
│   │   ├── ws_client.go             # WebSocket RPC v3 client
│   │   ├── helpers.go               # Shared utilities
│   │   ├── agents.go                # Agent CRUD
│   │   ├── providers.go             # Provider CRUD (API key masking)
│   │   ├── channels.go              # Channel CRUD (renamed from ChannelInstance)
│   │   ├── mcp_servers.go           # MCP server CRUD
│   │   ├── skills.go                # Skill observe/update (not deletable)
│   │   ├── tools.go                 # Tool CRUD (renamed from CustomTool)
│   │   ├── cron_jobs.go             # Cron job CRUD (WS, deletable)
│   │   ├── teams.go                 # Team CRUD (WS, deletable)
│   │   └── tts_config.go            # TTS config (WS, not deletable)
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
│   └── tui/                         # Interactive terminal UI (k9s-style)
│       ├── app.go                   # Main app, layout, refresh loop
│       ├── model.go                 # Thread-safe shared state
│       ├── keybindings.go           # Vim-style mode dispatch
│       └── views/
│           ├── resource_table.go    # Resource list with status coloring
│           ├── resource_detail.go   # YAML view with syntax highlighting
│           └── drift_view.go        # Field-level drift diff
└── examples/
    ├── minimal.yaml                 # Minimal manifest example (camelCase)
    ├── production.yaml              # Production manifest example (camelCase)
    └── local-dev.yaml               # Full-featured example (4 providers, agents, channels, tools, crons)
```

## Key Design Decisions

### Dual Transport
- **HTTP REST**: Primary for Provider, Agent, Channel, MCPServer, Skill, Tool (support Create/Update/Delete/List)
- **WebSocket RPC v3**: CronJob, Team, TTSConfig (no HTTP endpoints in GoClaw; support Create/Update/Delete/List)
- WS connection is lazy-initialized on first WS resource access

### No Local State
GoClaw API is the single source of truth. GCPlane carries no local state (SQLite removed). Every reconciliation queries live state, ensuring accuracy and simplifying deployments.

### Natural Key Resolution
GoClaw uses UUIDs internally. GCPlane uses human-readable natural keys (`name` field). Resolution: observe (list all) → filter by `name` → extract UUID for mutations.

### Dependency Ordering
Resources processed in dependency order: Provider → Agent → Skill → MCPServer → Tool → Channel → CronJob → Team → TTSConfig. Prune deletes in reverse order (safe cascading).

### Prune Safety
- Prune is opt-in (requires `--prune` flag or manifest `prune: true`)
- Only deletes resources marked with `gcplane.io/managed: true` (GCPlane-owned)
- Skill and TTSConfig are excluded (GoClaw manages these)
- Deletes happen in reverse dependency order to prevent cascade failures
- Continue-on-error per-resource; one failure doesn't block others

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

### Top (interactive dashboard)
1. Load + validate manifest
2. Resolve connection config (flags > env > manifest)
3. Create tview app with shared state model (thread-safe)
4. Start refresh goroutine on `--interval` (default 10s):
   a. List all resources from GoClaw
   b. Compute status (InSync, Drifted, Missing, Error, Extra)
   c. Update shared model (atomic write)
5. Render resource table with status coloring
6. Handle vim-style keybindings:
   - j/k: navigate, g/G: jump, Enter: show YAML, d: show drift, /: search, :: commands
   - 0-9: filter by kind, r: refresh, ?: help, q: quit
7. Detail views: YAML syntax highlighting, field-level diff on drift
8. Graceful shutdown on Ctrl+C (close WS connection, cleanup tview)
