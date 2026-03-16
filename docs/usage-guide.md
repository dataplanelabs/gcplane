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
    key: anthropic
    spec:
      displayName: "Anthropic"
      baseUrl: https://api.anthropic.com
      apiKey: ${ANTHROPIC_API_KEY}
      models:
        - claude-sonnet-4-20250514

  - kind: Agent
    key: assistant
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
| `diff` | Quick drift detection (coming soon) |
| `export` | Export GoClaw state as YAML (coming soon) |
| `serve` | Continuous reconciliation service (coming soon) |
| `version` | Print version |

## Global Flags

| Flag | Env Var | Description |
|------|---------|-------------|
| `-f, --file` | — | Manifest file or directory |
| `--endpoint` | `GCPLANE_ENDPOINT` | GoClaw endpoint URL |
| `--token` | `GCPLANE_TOKEN` | GoClaw auth token |
| `-v, --verbose` | — | Verbose output |

**Priority**: CLI flags > environment variables > manifest `connection` block.

## Manifest Reference

### Supported Resource Kinds

| Kind | Transport | Operations |
|------|-----------|------------|
| `Provider` | HTTP | create, update |
| `Agent` | HTTP | create, update |
| `ChannelInstance` | HTTP | create, update |
| `MCPServer` | HTTP | create, update |
| `Skill` | HTTP | update only |
| `CustomTool` | HTTP | create, update |
| `CronJob` | WebSocket | create, update |
| `Team` | WebSocket | create, update |
| `TTSConfig` | WebSocket | create, update |

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

## State Tracking

GCPlane stores reconciliation state in `.gcplane/state.db` (SQLite). This tracks:
- External IDs (GoClaw UUIDs)
- Spec hashes to detect local manifest changes
- Sync status and timestamps
