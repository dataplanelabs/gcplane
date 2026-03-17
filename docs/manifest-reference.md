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

## Tool Configuration

Agents support per-agent tool policies via `toolsConfig` in the spec:

```yaml
- kind: Agent
  name: dev-lead
  spec:
    toolsConfig:
      profile: coding          # "coding", "minimal", "all"
      exec:
        enabled: true
        timeout: 30            # seconds
      webFetch:
        enabled: true
      fileRead:
        enabled: true
      subagent:
        enabled: true
        maxDepth: 3
```

### Profiles

| Profile | Tools Enabled |
|---------|--------------|
| `coding` | exec, file read/write, web fetch, subagent |
| `minimal` | web fetch only |
| `all` | all built-in tools |

Individual tool overrides take precedence over profile defaults.

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

## Config Auto-Discovery

When `-f` / `--file` is not provided and `GCPLANE_CONFIG` env is unset, gcplane searches the working directory for a manifest in this order:

1. `gcplane.yaml`
2. `gcplane.yml`
3. `.gcplane.yaml`

This means you can simply run `gcplane plan` in a directory that contains a `gcplane.yaml` without specifying the path explicitly.

## Commands

```bash
gcplane init                         # generate starter manifest interactively
gcplane validate -f manifest.yaml    # check syntax (offline)
gcplane plan -f manifest.yaml        # dry-run diff
gcplane apply -f manifest.yaml       # apply changes
gcplane status -f manifest.yaml      # quick resource count + sync state
gcplane destroy                      # remove all gcplane-managed resources
gcplane serve -f manifest.yaml       # continuous reconciliation
```

### init

Interactive wizard that scaffolds a `gcplane.yaml` with basic Provider + Agent and creates `.env.example`.

```bash
gcplane init
```

Prompts for deployment name and GoClaw endpoint. Will not overwrite an existing `gcplane.yaml`.

### status

Quick one-shot health check. Shows resource counts and sync state without detailed diffs.

```bash
gcplane status                    # auto-discovers manifest
gcplane status -f manifest.yaml   # explicit path
```

Output:

```
GCPlane Status — my-deployment

  Resources:  3 total
  In Sync:    2
  Drifted:    1

  Provider     1
  Agent        2

  Run gcplane plan for details.
```

### destroy

Removes all resources from GoClaw that were created by gcplane (`created_by=gcplane`). Deletes in reverse dependency order. Resources created via the UI or other tools are not affected.

```bash
gcplane destroy --endpoint http://localhost:18790 --token $GOCLAW_TOKEN
gcplane destroy -f manifest.yaml              # use manifest for connection
gcplane destroy -f manifest.yaml --dry-run    # preview without deleting
gcplane destroy -f manifest.yaml --auto-approve
gcplane destroy -f manifest.yaml --backup state.yaml --log-file audit.jsonl
```

| Flag | Description |
|------|-------------|
| `--dry-run` | Preview resources that would be deleted, then exit |
| `--auto-approve` | Skip interactive confirmation prompt |
| `--backup <file>` | Export current state to YAML before destroying |
| `--log-file <file>` | Append JSON audit entry to file after destroy |

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
