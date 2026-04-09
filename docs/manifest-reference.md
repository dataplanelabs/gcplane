# Manifest Reference

## Version Compatibility

| GCPlane | Requires | RPC |
|---------|----------|-----|
| v1.3.0+ | GoClaw 3.x | v3 |
| v1.0.0–v1.2.x | GoClaw 2.x | v3 |

### GoClaw v3 Breaking Changes

- **Agent `agentType: open` deprecated** — v3 auto-converts to `predefined`. Use `predefined` in new manifests.
- **12 agent fields promoted** from `other_config` JSONB to dedicated columns. 7 are now observable (compared during reconcile), 5 remain write-only (complex JSONB configs).
- **New channel types**: `facebook` (Facebook Messenger) and `pancake` (Pancake CRM hub).

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
  tenantId: ${GOCLAW_TENANT_ID}  # optional — scope all resources to this tenant
resources:
  - kind: Provider
    name: my-provider
    spec: { ... }
```

## Resource Kinds

| Kind | Transport | Deletable | Description |
|------|-----------|-----------|-------------|
| `Tenant` | HTTP | Yes | Tenant definition (system scope only) |
| `Provider` | HTTP | Yes | LLM provider (Anthropic, OpenAI, etc.) |
| `Agent` | HTTP | Yes | AI agent with model + config |
| `Channel` | HTTP | Yes | Messaging channel (Telegram, Discord, etc.) |
| `MCPServer` | HTTP | Yes | MCP tool server |
| `Skill` | HTTP | No | Agent skill (update only, auto-discovered, GoClaw-managed) |
| `BuiltinToolConfig` | HTTP | Yes | Per-tenant builtin tool enable/disable |
| `SkillConfig` | HTTP | Yes | Per-tenant skill enable/disable |
| `SystemConfig` | HTTP | Yes | Per-tenant key-value system settings |
| `MCPCredentials` | HTTP | Yes | Per-user MCP server credentials |
| `SecureCLI` | HTTP | Yes | Secure CLI binary configs (v1.1.0+) |
| `SecureCLIGrant` | HTTP | Yes | Per-agent CLI overrides, child resource (v1.1.0+) |
| `CronJob` | WebSocket | Yes | Scheduled task |
| `AgentTeam` | WebSocket | Yes | Agent team |

Resources are applied in dependency order: Tenant → Provider → Agent → Skill → BuiltinToolConfig → SkillConfig → SystemConfig → MCPServer → MCPCredentials → Channel → CronJob → SecureCLI → SecureCLIGrant → AgentTeam. Prune deletes in reverse order.

**Note:** Skill is managed by GoClaw and cannot be deleted by gcplane. BuiltinToolConfig, SkillConfig, SystemConfig, MCPCredentials, and SecureCLIGrant are not enumerable for prune.

## Channel Configuration

Channel credentials are stored in a `credentials` object (nested structure, not top-level fields). The structure varies by channel type:

### Telegram Channel
```yaml
- kind: Channel
  name: telegram-main
  spec:
    displayName: "Telegram Bot"
    channelType: telegram
    agentKey: bot-agent
    enabled: true
    credentials:
      token: ${TELEGRAM_BOT_TOKEN}
    config:
      dmPolicy: open       # or "closed"
      groupPolicy: open    # or "closed"
```

### Slack Channel
```yaml
- kind: Channel
  name: slack-main
  spec:
    displayName: "Slack Bot"
    channelType: slack
    agentKey: bot-agent
    enabled: true
    credentials:
      botToken: ${SLACK_BOT_TOKEN}
      appToken: ${SLACK_APP_TOKEN}
    config:
      dmPolicy: open
```

### Facebook Messenger Channel (v3)
```yaml
- kind: Channel
  name: facebook-main
  spec:
    displayName: "Facebook Messenger"
    channelType: facebook
    agentKey: bot-agent
    enabled: true
    credentials:
      pageAccessToken: ${FB_PAGE_ACCESS_TOKEN}
      appSecret: ${FB_APP_SECRET}
      verifyToken: ${FB_VERIFY_TOKEN}
```

### Pancake Channel (v3)
```yaml
- kind: Channel
  name: pancake-crm
  spec:
    displayName: "Pancake CRM"
    channelType: pancake
    agentKey: bot-agent
    enabled: true
    credentials:
      apiKey: ${PANCAKE_API_KEY}
```

**Note:** The `credentials` field is write-only (excluded from comparison). Tokens are not returned by GoClaw during observe, preventing phantom diffs.

## CronJob Configuration

CronJobs use WebSocket RPC (camelCase). Fields `deliver`, `deliverChannel`, `deliverTo`, `stateless`, `wakeHeartbeat`, and `deleteAfterRun` are observable (compared during reconcile). The `message` and `agentKey` fields are write-only.

```yaml
- kind: CronJob
  name: daily-report
  spec:
    schedule:
      kind: cron             # "cron", "every", or "at"
      expr: "0 9 * * 1-5"   # cron expression (kind=cron)
      tz: Asia/Saigon        # IANA timezone
    message: "Generate daily report and summarize findings"
    agentKey: report-bot     # resolved to agentId UUID
    enabled: true
    deliver: true            # deliver result to channel
    deliverChannel: telegram # target channel name
    deliverTo: "@admin"      # recipient within channel
    stateless: true          # skip session save (default true for new jobs)
    wakeHeartbeat: false     # wake agent via heartbeat before run
    deleteAfterRun: false    # remove job after first execution
```

| Field | Type | Observable | Description |
|-------|------|-----------|-------------|
| `schedule` | object | Yes | When to run (kind, expr/everyMs/atMs, tz) |
| `enabled` | bool | Yes | Active/paused |
| `message` | string | No (write-only) | Prompt sent to agent |
| `agentKey` | string | No (write-only) | Agent name, resolved to UUID |
| `deliver` | bool | Yes | Deliver output to channel |
| `deliverChannel` | string | Yes | Target channel for delivery |
| `deliverTo` | string | Yes | Recipient within channel |
| `stateless` | bool | Yes | Fresh session per run (saves tokens) |
| `wakeHeartbeat` | bool | Yes | Send heartbeat before execution |
| `deleteAfterRun` | bool | Yes | One-shot job (auto-deletes) |

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

## Agent Fields (v3 Promoted)

GoClaw v3 promoted 12 fields from the `other_config` JSONB bag to dedicated columns. These are now directly manageable from manifests.

### Observable Fields (compared during reconcile)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `emoji` | string | `""` | Agent avatar emoji |
| `agentDescription` | string | `""` | Agent description / summoning prompt |
| `thinkingLevel` | string | `"off"` | Reasoning effort: `off`, `low`, `medium`, `high` |
| `maxTokens` | int | `0` | Max output tokens (0 = provider default) |
| `selfEvolve` | bool | `false` | Enable agent self-evolution |
| `skillEvolve` | bool | `false` | Enable skill evolution |
| `skillNudgeInterval` | int | `0` | Skill nudge interval in messages |

These fields are optional. Omitting them from the manifest causes no drift — only fields present in the manifest spec are compared.

### Write-Only Fields (set but not compared)

| Field | Type | Description |
|-------|------|-------------|
| `reasoningConfig` | object | Advanced reasoning settings (override mode, budget tokens) |
| `workspaceSharing` | object | Workspace sharing policy (memory, knowledge graph toggles) |
| `chatgptOauthRouting` | object | ChatGPT OAuth routing config |
| `shellDenyGroups` | object | Shell command deny groups |
| `kgDedupConfig` | object | Knowledge graph dedup configuration |

```yaml
- kind: Agent
  name: smart-agent
  spec:
    provider: anthropic
    model: claude-sonnet-4-6
    emoji: "🧠"
    agentDescription: "Research assistant with reasoning"
    thinkingLevel: high
    maxTokens: 8192
    selfEvolve: true
    contextWindow: 200000
    maxToolIterations: 50
```

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

The optional `connection.tenantId` field scopes all API requests to a specific tenant via the `X-GoClaw-Tenant-Id` header. Not needed when using tenant-bound API keys (auto-scoped).

## Multi-Tenant Support

GCPlane supports two multi-tenant deployment models:

**1. Single GoClaw, multiple tenants** (recommended for SaaS):
```
tenants/
├── _system/           # system-level key — manages Tenant resources
│   └── manifest.yaml
├── acme-corp/         # tenant-bound key — manages resources within tenant
│   └── manifest.yaml
└── globex-inc/
    └── manifest.yaml
```

**2. Multiple GoClaw instances** (one per tenant):
Each subdirectory connects to a different GoClaw endpoint.

Run with: `gcplane serve --tenants-dir tenants/`

### Tenant Resource

Create tenants declaratively (requires system-level API key):

```yaml
- kind: Tenant
  name: acme-corp      # slug — kebab-case identifier
  spec:
    displayName: "Acme Corporation"
```

### Per-Tenant Config Resources

```yaml
# Enable/disable builtin tools for this tenant
- kind: BuiltinToolConfig
  name: exec
  spec:
    enabled: true

# Enable/disable skills for this tenant
- kind: SkillConfig
  name: my-skill-slug
  spec:
    enabled: false

# Per-user MCP server credentials
- kind: MCPCredentials
  name: github-mcp      # MCP server name
  spec:
    userId: "service-account"
    credentials:
      apiKey: ${GITHUB_API_KEY}
```

## Secure CLI Configuration

Manage secure CLI binary access and per-agent overrides:

```yaml
# Define a secure CLI binary
- kind: SecureCLI
  name: kubectl
  spec:
    binaryName: kubectl
    isGlobal: true
    denyArgs: ["delete", "exec"]
    timeoutSeconds: 30
    tips: "Use --dry-run for safety"
    enabled: true
    env:
      KUBECONFIG: /etc/kubernetes/config

# Per-agent CLI grant (overrides parent SecureCLI settings)
- kind: SecureCLIGrant
  name: kubectl--assistant
  spec:
    binaryName: kubectl
    agentKey: assistant
    timeoutSeconds: 60
    enabled: true
    # other fields inherit from SecureCLI if not specified
```

**Reference Validation**: SecureCLIGrant specs validate that `agentKey` references an Agent and `binaryName` references a SecureCLI resource. Missing references fail validation with clear errors.

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
- Skill is excluded (GoClaw-managed, cannot be deleted)
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
