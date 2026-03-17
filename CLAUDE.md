# GCPlane

Declarative GitOps control plane for GoClaw. Manages AI agents, providers, channels, MCP servers, cron jobs, and teams through YAML manifests.

## Core Principles
- **Deploy-Anywhere**: Single binary for local, VPS, k8s. No platform lock-in.
- **Minimal Dependencies**: Under 10 deps. Stdlib preferred. No SDKs for simple HTTP calls.
- **Self-Contained**: Config via env vars + YAML manifest only.

## Tech Stack
- Go 1.25, Cobra CLI, gorilla/websocket, gopkg.in/yaml.v3
- No ORM, no heavy deps — 5 total dependencies
- GoClaw API: HTTP REST + WebSocket RPC v3

## Architecture
```
cmd/              — CLI commands (validate, plan, apply, diff, export, serve, destroy, init, status, top)
internal/
  manifest/       — YAML loader, validator, composites, labels, field config
  reconciler/     — Observe→Compare→Act engine with ReconcileOpts (DryRun, Prune)
  provider/goclaw — GoClaw API client (HTTP + WS) for 9 resource types
  keyconv/        — camelCase↔snake_case key translation
  controller/     — Reconcile loop, status tracker, tenant manager
  server/         — HTTP endpoints (health, metrics, status, sync, webhook)
  source/         — Manifest sources (file with SHA256, git with clone/fetch)
  display/        — Colored terminal output (plan, diff, prune warning)
  secrets/        — ${ENV_VAR} and file:// resolution
  tui/            — Interactive terminal UI (k9s-style resource browser with vim keybindings)
```

## Key Patterns
- Manifest uses camelCase (k8s convention), provider translates to snake_case for GoClaw API
- `WriteOnlyFields` in `manifest/field_config.go` — fields excluded from comparison (secrets, grants, tokens)
- `stripInternal` in `provider/goclaw/helpers.go` — removes API-internal fields from observe results
- Prune: `--prune` flag, deletes in reverse `DeleteOrder()`, only `created_by=gcplane`
- Composites: `CompositeDefinition` expanded during load via Go `text/template`

## Testing
```bash
make test        # unit tests
make test-e2e    # reset GoClaw + test all commands
make reset       # wipe GoClaw + re-apply manifest
make serve       # continuous reconciliation
```

## Local Dev
```bash
cp .env.example .env  # fill in credentials
make setup            # start GoClaw docker + apply config
```

## GoClaw Compatibility
Tested against `ghcr.io/nextlevelbuilder/goclaw:1.2.0-full`
