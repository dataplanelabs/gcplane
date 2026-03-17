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

| Kind | Transport | Deletable | Description |
|------|-----------|-----------|-------------|
| `Provider` | HTTP | Yes | LLM provider (Anthropic, OpenAI, etc.) |
| `Agent` | HTTP | Yes | AI agent with model + config |
| `Channel` | HTTP | Yes | Messaging channel (Telegram, Discord, etc.) |
| `MCPServer` | HTTP | Yes | MCP tool server |
| `Skill` | HTTP | No | Agent skill (update only, auto-discovered, GoClaw-managed) |
| `Tool` | HTTP | Yes | Custom tool definition |
| `CronJob` | WebSocket | Yes | Scheduled task |
| `AgentTeam` | WebSocket | Yes | Agent team |
| `TTSConfig` | WebSocket | No | Text-to-speech settings (GoClaw-managed) |

Resources are applied in dependency order: Provider → Agent → Skill → MCPServer → Tool → Channel → CronJob → Team → TTSConfig. Prune deletes in reverse order.

**Note:** Skill and TTSConfig are managed by GoClaw and cannot be deleted by gcplane. They are excluded from prune operations.

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

## Prune (Delete Orphaned Resources)

By default, gcplane only creates and updates resources. Prune mode detects and deletes resources that are:
- Present in GoClaw
- Marked with `gcplane.io/managed: true`
- NOT in the current manifest

### Safe Deletion

Enable prune with the `--prune` flag:

```bash
gcplane plan -f manifest.yaml --prune

gcplane apply -f manifest.yaml --prune
```

**Safety guarantees:**
- Prune is opt-in (requires explicit `--prune` flag or `prune: true` in manifest)
- Only deletes gcplane-owned resources (marked with `gcplane.io/managed: true`)
- Skill and TTSConfig are excluded (GoClaw-managed, cannot be deleted)
- Deletes happen in reverse dependency order (safe cascading)
- Shows warning when deletions > 0: `N to create, N to update, N to delete`
- Continue-on-error: one delete failure doesn't block others

### Reference Validation

gcplane pre-validates all resource references before reconciliation. Missing referenced resources fail validation with clear error messages.

## Serve Mode

Long-running GitOps controller with periodic reconciliation.

```bash
# Watch local file
gcplane serve -f manifest.yaml --interval 30s

# Watch git repo
gcplane serve --repo git@github.com:org/config.git \
  --branch main --path manifest.yaml --interval 30s

# Enable prune in serve mode
gcplane serve -f manifest.yaml --prune --interval 30s
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
