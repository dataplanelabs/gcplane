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

## Version & Requirements

**GCPlane v0.7.2+** requires **GoClaw 1.2.0+** with RPC v3 support.

| Requirement | Version |
|-------------|---------|
| Go | 1.25.8+ (build only) |
| GoClaw | 1.2.0+ (runtime) |
| Docker | (optional, for local dev) |

Check your GCPlane version:
```bash
gcplane version
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

| Command | Description | Since |
|---------|-------------|-------|
| `init` | Generate a starter manifest interactively (supports provider type selection: anthropic, openai, openrouter, custom) | v0.6.0 |
| `validate` | Validate manifest schema (no GoClaw connection) | v0.1.0 |
| `plan` | Show changes required (dry-run) | v0.1.0 |
| `apply` | Apply manifest to reach desired state | v0.1.0 |
| `status` | Quick resource count and sync state summary | v0.1.0 |
| `destroy` | Remove all gcplane-managed resources from GoClaw | v0.4.0 |
| `serve` | Continuous reconciliation service with file/git sources | v0.1.0 |
| `top` | Interactive TUI for monitoring GoClaw resources (k9s-style dashboard) | v0.7.0 |
| `diff` | Show drift between manifest and live state | v0.2.0 |
| `export` | Export GoClaw state as YAML manifest | v0.2.0 |
| `version` | Print version and check for updates | v0.6.0 |

## Global Flags

| Flag | Env Var | Description |
|------|---------|-------------|
| `-f, --file` | — | Manifest file or directory |
| `--endpoint` | `GCPLANE_ENDPOINT` | GoClaw endpoint URL |
| `--token` | `GCPLANE_TOKEN` | GoClaw auth token |
| `-v, --verbose` | — | Verbose output |

**Priority**: CLI flags > environment variables > manifest `connection` block.

## Serve Environment Variables

| Env Var | Description |
|---------|-------------|
| `GCPLANE_WEBHOOK_URL` | Webhook URL for drift notifications |
| `GCPLANE_WEBHOOK_FORMAT` | Payload format: `slack` (default), `discord`, `googlechat`, `teams`, `telegram` |
| `GCPLANE_LOG_FORMAT` | Log format: `text` (default) or `json` for structured output |

## Plan & Apply Flags

| Flag | Description |
|------|-------------|
| `--prune` | Delete resources removed from manifest (default: false) |
| `--auto-approve` | Skip confirmation prompt (apply only) |
| `--log-file` | Write audit log to file (apply and destroy commands) |

**Prune Safety**: Prune is opt-in to prevent accidental deletions. Only deletes gcplane-owned resources.

## Destroy Flags

| Flag | Description |
|------|-------------|
| `--backup` | Auto-export state to manifest snapshot before destruction |
| `--log-file` | Write audit log to file |

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

## Version Command

Check your installed GCPlane version and receive notifications about updates:

```bash
gcplane version
```

Output:
```
GCPlane v0.7.2
Update available: v0.8.0 (released 2026-03-24)
```

The update checker:
- Runs in background (non-blocking)
- Checks GitHub API for new releases
- Caches result for 24 hours
- Prints notification to stderr

To disable update checks, set `GCPLANE_SKIP_UPDATE_CHECK=1`.

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

## Top (Interactive Dashboard)

k9s-style terminal UI for real-time monitoring of GoClaw resources:

```bash
# Monitor with default 10s refresh
gcplane top -f gcplane.yaml

# Custom refresh interval
gcplane top -f manifest.yaml --interval 5s

# Specify endpoint (overrides manifest)
gcplane top -f manifest.yaml --endpoint http://localhost:8080
```

### Features

- **Resource Browser**: Browse all 9 resource kinds (Provider, Agent, Channel, MCPServer, Skill, Tool, CronJob, AgentTeam, TTSConfig)
- **Status Coloring**: InSync (green), Drifted (yellow), Missing/Error (red), Extra (blue)
- **YAML View**: Press Enter to view full resource YAML with syntax highlighting
- **Drift Details**: Press `d` to see field-level drift comparison
- **Vim Keybindings**: j/k navigate, g/G jump to start/end, q quit, ? help, : commands, / search
- **Kind Filtering**: Number keys 0-9 or type `:agent`, `:provider`, `:mcp`, `:cron`, `:team`, `:tts`, `:all`
- **Auto-Refresh**: Configurable interval (default 10s), press `r` for manual refresh
- **Search**: Press `/` to filter by resource name (case-insensitive)

### Keybindings

| Key | Action |
|-----|--------|
| `j`/`k` | Navigate up/down |
| `g` | Jump to start |
| `G` | Jump to end |
| `Enter` | View resource YAML details |
| `d` | Show drift diff |
| `r` | Refresh resources |
| `0-9` | Filter by kind (0=all, 1=provider, 2=agent, etc.) |
| `:` | Command mode |
| `/` | Search by name |
| `?` | Show help |
| `q` | Quit |

### Top Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-f, --file` | — | Manifest file or directory |
| `--interval` | `10s` | Refresh interval |
| `--endpoint` | — | GoClaw endpoint URL (overrides manifest) |
| `--token` | — | GoClaw auth token (overrides manifest/env) |

## Deployment

GCPlane is a single binary — deploy anywhere.

### Local (docker compose)

```bash
# 1. Copy env file and fill in credentials
cp .env.example .env

# 2. Start gcplane (builds from source, watches examples/local-dev.yaml)
docker compose up -d

# 3. Check health
curl http://localhost:8480/healthz

# 4. View logs
docker compose logs -f

# 5. Stop
docker compose down
```

To use a custom manifest, edit `docker-compose.yaml` volumes to mount your file:

```yaml
volumes:
  - ./my-manifest.yaml:/config/manifest.yaml:ro
```

### VPS (binary / Docker)

```bash
# Direct binary
gcplane serve -f /etc/gcplane/manifest.yaml --interval 30s

# Docker (single container)
docker run -v /path/to/manifest.yaml:/config/manifest.yaml \
  -e GOCLAW_TOKEN=your-token \
  -p 8480:8480 \
  ghcr.io/dataplanelabs/gcplane:latest \
  serve -f /config/manifest.yaml --interval 30s
```

### Kubernetes (kustomize)

```bash
# Dev environment
kubectl apply -k deploy/overlays/dev

# Staging
kubectl apply -k deploy/overlays/staging

# Production (2 replicas, higher resources)
kubectl apply -k deploy/overlays/prod
```

Edit `deploy/base/configmap.yaml` with your manifest, create a Secret named `gcplane-secrets` with your env vars:

```bash
kubectl create secret generic gcplane-secrets \
  --from-literal=GOCLAW_TOKEN=your-token \
  --from-literal=ANTHROPIC_API_KEY=sk-ant-...
```
