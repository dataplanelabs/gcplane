# GCPlane Usage Guide

## Installation

```bash
go install github.com/dataplanelabs/gcplane@latest
```

Or build from source:
```bash
git clone https://github.com/dataplanelabs/gcplane.git
cd gcplane
go build -o gcplane .
```

## Quick Start

### 1. Create a manifest

```yaml
# gcplane.yaml
apiVersion: gcplane.io/v1
kind: Manifest

metadata:
  name: my-deployment
  environment: dev

connection:
  endpoint: http://localhost:18790
  token: ${GOCLAW_TOKEN}

resources:
  - kind: Provider
    name: anthropic
    spec:
      displayName: "Anthropic"
      baseUrl: https://api.anthropic.com
      apiKey: ${ANTHROPIC_API_KEY}
      models:
        - claude-sonnet-4-20250514

  - kind: Agent
    name: assistant
    spec:
      displayName: "Assistant"
      provider: anthropic
      model: claude-sonnet-4-20250514
      agentType: open
      status: active
```

### 2. Set environment variables

```bash
export GOCLAW_TOKEN="your-goclaw-token"
export ANTHROPIC_API_KEY="sk-ant-..."
```

### 3. Validate the manifest

```bash
gcplane validate -f gcplane.yaml
```

### 4. Preview changes (dry-run)

```bash
gcplane plan -f gcplane.yaml
```

### 5. Apply changes

```bash
gcplane apply -f gcplane.yaml

# Skip confirmation prompt
gcplane apply -f gcplane.yaml --auto-approve
```

## Commands

| Command | Description |
|---------|-------------|
| `validate` | Validate manifest schema (no GoClaw connection) |
| `plan` | Show changes required (dry-run) |
| `apply` | Apply manifest to reach desired state |
| `serve` | Continuous reconciliation service with file/git sources |
| `diff` | Quick drift detection (coming soon) |
| `export` | Export GoClaw state as YAML (coming soon) |
| `version` | Print version |

## Global Flags

| Flag | Env Var | Description |
|------|---------|-------------|
| `-f, --file` | — | Manifest file or directory |
| `--endpoint` | `GCPLANE_ENDPOINT` | GoClaw endpoint URL |
| `--token` | `GCPLANE_TOKEN` | GoClaw auth token |
| `-v, --verbose` | — | Verbose output |

**Priority**: CLI flags > environment variables > manifest `connection` block.

## Plan & Apply Flags

| Flag | Description |
|------|-------------|
| `--prune` | Delete resources removed from manifest (default: false) |
| `--auto-approve` | Skip confirmation prompt (apply only) |

**Prune Safety**: Prune is opt-in to prevent accidental deletions. Only deletes gcplane-owned resources.

## Manifest Reference

### Supported Resource Kinds

| Kind | Transport | Operations |
|------|-----------|------------|
| `Provider` | HTTP | create, update, delete, list |
| `Agent` | HTTP | create, update, delete, list |
| `Channel` | HTTP | create, update, delete, list |
| `MCPServer` | HTTP | create, update, delete, list |
| `Skill` | HTTP | update only (auto-discovered) |
| `Tool` | HTTP | create, update, delete, list |
| `CronJob` | WebSocket | create, update, delete, list |
| `AgentTeam` | WebSocket | create, update, delete, list |
| `TTSConfig` | WebSocket | update only (GoClaw-managed) |

### Secret Resolution

Manifest values support secret references:

```yaml
# Environment variable
token: ${GOCLAW_TOKEN}

# File reference
apiKey: file:///path/to/secret.txt
```

### Directory Mode

Split manifests across multiple files in a directory:

```bash
gcplane plan -f ./manifests/
```

All `.yaml`/`.yml` files are merged. First file's `connection` and `metadata` win.

## Plan Output

GCPlane uses terraform-style colored output:

```
GCPlane Plan: 1 to create, 1 to update, 0 unchanged

+ Provider/anthropic
~ Agent/assistant
    model: "claude-haiku-4-5-20251001" → "claude-sonnet-4-20250514"

Plan: 1 to create, 1 to update, 0 unchanged.
```

- `+` (green) — resource will be created
- `~` (yellow) — resource will be updated, with field diffs
- `=` (dim) — no changes (verbose mode only)

## Serve Mode

Long-running GitOps controller with periodic reconciliation and health endpoints:

```bash
# Watch local manifest file
gcplane serve -f manifest.yaml --interval 30s

# Watch git repository (auto-pull on webhook)
gcplane serve --repo git@github.com:org/config.git \
  --branch main --path manifest.yaml --interval 30s

# Enable prune in serve mode
gcplane serve -f manifest.yaml --prune --interval 30s
```

Exposes HTTP endpoints on `--addr` (default `:8480`):
- `GET /healthz` — Liveness probe (always 200)
- `GET /readyz` — Readiness probe (200 after first sync)
- `GET /metrics` — Prometheus metrics (sync count, duration, last timestamp)
- `GET /api/v1/status` — Full sync status + per-resource state
- `POST /api/v1/sync` — Trigger immediate reconcile
- `POST /api/v1/webhook/git` — Git push webhook trigger (for CI/CD pipelines)
