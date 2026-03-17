# Manifest Reference

## Format

gcplane uses declarative YAML manifests following k8s conventions (camelCase keys, `apiVersion`/`kind`/`spec` pattern).

```yaml
apiVersion: gcplane.io/v1
kind: Manifest
metadata:
  name: deployment-name
  environment: dev|staging|production
connection:
  endpoint: http://localhost:18790
  token: ${GOCLAW_TOKEN}
resources:
  - kind: Provider
    name: my-provider
    spec: { ... }
```

## Resource Kinds

| Kind | Transport | Description |
|------|-----------|-------------|
| `Provider` | HTTP | LLM provider (Anthropic, OpenAI, etc.) |
| `Agent` | HTTP | AI agent with model + config |
| `Channel` | HTTP | Messaging channel (Telegram, Discord, etc.) |
| `MCPServer` | HTTP | MCP tool server |
| `Skill` | HTTP | Agent skill (update only, auto-discovered) |
| `Tool` | HTTP | Custom tool definition |
| `CronJob` | WebSocket | Scheduled task |
| `Team` | WebSocket | Agent team |
| `TTSConfig` | WebSocket | Text-to-speech settings |

Resources are applied in dependency order: Provider → Agent → Skill → MCPServer → Tool → Channel → CronJob → Team → TTSConfig.

## Secret Resolution

Spec values support two secret formats:

- **Environment variable:** `${ENV_VAR_NAME}` — resolved from shell environment
- **File reference:** `file:///path/to/secret` — reads file contents

## Connection Config

Priority: CLI flags > env vars > manifest.

| Source | Endpoint | Token |
|--------|----------|-------|
| CLI flag | `--endpoint` | `--token` |
| Env var | `GCPLANE_ENDPOINT` | `GCPLANE_TOKEN` |
| Manifest | `connection.endpoint` | `connection.token` |

## Commands

```bash
gcplane validate -f manifest.yaml    # check syntax (offline)
gcplane plan -f manifest.yaml        # dry-run diff
gcplane apply -f manifest.yaml       # apply changes
gcplane serve -f manifest.yaml       # continuous reconciliation
```

## Serve Mode

Long-running GitOps controller with periodic reconciliation.

```bash
# Watch local file
gcplane serve -f manifest.yaml --interval 30s

# Watch git repo
gcplane serve --repo git@github.com:org/config.git \
  --branch main --path manifest.yaml --interval 30s
```

Endpoints exposed at `--addr` (default `:8480`):

| Endpoint | Description |
|----------|-------------|
| `GET /healthz` | Liveness probe (always 200) |
| `GET /readyz` | Readiness probe (200 after first sync) |
| `GET /metrics` | Prometheus metrics |
| `GET /api/v1/status` | Sync status + per-resource state |
| `POST /api/v1/sync` | Trigger immediate reconcile |
| `POST /api/v1/webhook/git` | Git push webhook trigger |
