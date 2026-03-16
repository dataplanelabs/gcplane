# GCPlane System Architecture

## Overview

GCPlane is a GitOps-style control plane for managing GoClaw deployments. It reads declarative YAML manifests and reconciles them against the actual GoClaw state via HTTP REST and WebSocket RPC APIs.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────┐
│                       CLI Layer                          │
│  gcplane validate | plan | apply | diff | export | serve │
└──────────┬──────────────────────────────────────────────┘
           │
┌──────────▼──────────────────────────────────────────────┐
│                    Manifest Loader                        │
│  YAML parsing → validation → secret resolution            │
│  Supports: single file, directory merge, env vars, files  │
└──────────┬──────────────────────────────────────────────┘
           │
┌──────────▼──────────────────────────────────────────────┐
│                  Reconciler Engine                        │
│  Observe → Compare → Act                                  │
│  Dependency-ordered processing (Provider → Agent → ...)   │
│  Modes: dry-run (plan) | apply                            │
└──────────┬──────────────────────────────────────────────┘
           │
┌──────────▼──────────────────────────────────────────────┐
│                  GoClaw Provider                          │
│  ┌─────────────────┐    ┌──────────────────┐            │
│  │   HTTP Client    │    │   WS RPC Client   │            │
│  │  REST endpoints  │    │  v3 protocol       │            │
│  │  Provider, Agent │    │  CronJob, Team     │            │
│  │  Channel, MCP    │    │  TTSConfig          │            │
│  │  Skill, Tool     │    │                    │            │
│  └────────┬────────┘    └────────┬───────────┘            │
└───────────┼──────────────────────┼──────────────────────┘
            │                      │
┌───────────▼──────────────────────▼──────────────────────┐
│                   GoClaw Instance                        │
│              HTTP REST + WebSocket RPC                    │
└─────────────────────────────────────────────────────────┘
           │
┌──────────▼──────────────────────────────────────────────┐
│                   State Store (SQLite)                    │
│  Tracks: external IDs, spec hashes, sync status          │
│  Location: .gcplane/state.db                              │
└─────────────────────────────────────────────────────────┘
```

## Package Structure

```
gcplane/
├── main.go                          # Entry point
├── cmd/                             # CLI commands (Cobra)
│   ├── root.go                      # Root command + global flags
│   ├── config.go                    # Connection config resolution
│   ├── plan.go                      # Dry-run reconciliation
│   ├── apply.go                     # Apply with confirmation
│   ├── validate.go                  # Schema validation
│   ├── diff.go                      # Drift detection (stub)
│   ├── export.go                    # State export (stub)
│   └── serve.go                     # Continuous reconciliation (stub)
├── internal/
│   ├── manifest/                    # YAML manifest handling
│   │   ├── types.go                 # Manifest, Resource, ResourceKind
│   │   ├── loader.go                # File/directory loading + merging
│   │   └── validate.go              # Schema validation rules
│   ├── reconciler/                  # Observe→Compare→Act engine
│   │   ├── types.go                 # Plan, Change, FieldDiff, ApplyResult
│   │   ├── engine.go                # Reconciliation logic + secret resolution
│   │   └── compare.go              # Deep spec comparison
│   ├── provider/goclaw/             # GoClaw API provider
│   │   ├── provider.go              # Provider struct + routing
│   │   ├── http_client.go           # Authenticated HTTP client
│   │   ├── ws_client.go             # WebSocket RPC v3 client
│   │   ├── helpers.go               # Shared utilities
│   │   ├── agents.go                # Agent CRUD
│   │   ├── providers.go             # Provider CRUD (API key masking)
│   │   ├── channels.go              # Channel instance CRUD
│   │   ├── mcp_servers.go           # MCP server CRUD
│   │   ├── skills.go                # Skill observe/update
│   │   ├── custom_tools.go          # Custom tool CRUD
│   │   ├── cron_jobs.go             # Cron job CRUD (WS)
│   │   ├── teams.go                 # Team CRUD (WS)
│   │   └── tts_config.go            # TTS config (WS)
│   ├── secrets/                     # Secret resolution
│   │   └── resolver.go              # ${ENV}, file://, SOPS support
│   ├── state/                       # Reconciliation state persistence
│   │   ├── store.go                 # Store interface
│   │   └── sqlite.go                # SQLite implementation
│   ├── display/                     # Terminal output formatting
│   │   └── plan.go                  # Terraform-style colored diff
│   └── server/                      # HTTP server (stub)
└── examples/
    ├── minimal.yaml                 # Minimal manifest example
    └── production.yaml              # Production manifest example
```

## Key Design Decisions

### Dual Transport
- **HTTP REST**: Primary transport for Provider, Agent, ChannelInstance, MCPServer, Skill, CustomTool
- **WebSocket RPC v3**: Fallback for CronJob, Team, TTSConfig (no HTTP endpoints in GoClaw)
- WS connection is lazy-initialized on first WS resource access

### Natural Key Resolution
GoClaw uses UUIDs internally. GCPlane uses human-readable natural keys (name, agentKey). Resolution pattern: list all → filter by natural key → extract UUID for mutations.

### Dependency Ordering
Resources are processed in dependency order: Provider → Agent → Skill → MCPServer → CustomTool → ChannelInstance → CronJob → Team → TTSConfig

### API Key Masking
GoClaw returns `"***"` for sensitive fields. The comparator skips masked fields to avoid false-positive diffs. On update, the full key from the manifest is always sent.

### Secret Resolution
Manifest values support `${ENV_VAR}` substitution and `file://path` references. Secrets are resolved at reconciliation time, not at load time.

## Data Flow

### Plan (dry-run)
1. Load + validate manifest
2. Resolve connection config (flags > env > manifest)
3. For each resource (in dependency order):
   a. Resolve secrets in spec
   b. Observe current state from GoClaw
   c. Compare desired vs actual (skip masked fields)
   d. Record: create / update / noop
4. Display colored diff

### Apply
1. Same as plan
2. Show diff, prompt for confirmation
3. Execute creates/updates via provider
4. Display results
