# Multi-Tenant Structure

## Overview

GCPlane v1.0.0 supports two multi-tenant deployment models:

1. **Single GoClaw, multiple tenants** (SaaS model, recommended)
   - One GoClaw instance with tenant isolation via `connection.tenantId` header
   - Tenant CRUD via `Tenant` resource kind
   - Per-tenant configs: `BuiltinToolConfig`, `SkillConfig`, `MCPCredentials`, `SystemConfig`
   - GoClaw 2.x required

2. **Multiple GoClaw instances** (federated model)
   - Each tenant = dedicated GoClaw endpoint
   - Traditional isolated deployment model

## Directory Layout (SaaS Model)

```
goclaw-config/
├── _system/                           # system-level (creates tenants)
│   ├── connection.yaml                # system API key
│   └── manifest.yaml
├── acme-corp/                         # tenant-scoped
│   ├── connection.yaml                # tenant API key + tenantId
│   └── manifest.yaml
└── globex-inc/
    ├── connection.yaml
    └── manifest.yaml
```

Run with: `gcplane serve --tenants-dir ./goclaw-config/`

## Isolation Model (GoClaw 2.x)

| Boundary | Isolation Level | Notes |
|----------|----------------|-------|
| **Tenant** | Full (API-enforced) | Single GoClaw instance, tenant scope in header |
| **Environment** | Logical | Via subdirectories or separate manifests |
| **Org unit** | Logical | Resource labels + naming conventions |

## Tenant Resource

Create tenants declaratively via `Tenant` resource (requires system API key):

```yaml
- kind: Tenant
  name: acme-corp          # kebab-case slug
  spec:
    displayName: "Acme Corporation"
```

System scope only. Not filterable by `connection.tenantId` (no self-reference).

## Per-Tenant Config Resources

After creating a tenant, use a tenant-bound API key to configure:

```yaml
# Enable/disable builtin tools (exec, webFetch, etc.)
- kind: BuiltinToolConfig
  name: exec
  spec:
    enabled: true

# Enable/disable skills
- kind: SkillConfig
  name: my-skill-slug
  spec:
    enabled: false

# Per-tenant system settings (v1.0.0+)
- kind: SystemConfig
  name: feature-flags
  spec:
    key1: value1
    key2: value2

# Per-user MCP server credentials
- kind: MCPCredentials
  name: github-mcp
  spec:
    userId: "user@example.com"
    credentials:
      apiKey: ${GITHUB_API_KEY}

# Secure CLI binaries (v1.1.0+)
- kind: SecureCLI
  name: kubectl
  spec:
    binaryName: kubectl
    isGlobal: true
    denyArgs: ["delete", "exec"]

# Per-agent CLI overrides (v1.1.0+, non-enumerable)
- kind: SecureCLIGrant
  name: kubectl--dev-assistant
  spec:
    binaryName: kubectl
    agentKey: dev-assistant
    timeoutSeconds: 60
```

## Tenant Isolation Policy

GCPlane enforces **strict tenant separation** — cross-tenant operations are not supported by design. Each tenant is a fully isolated unit:

- Resources in tenant A **cannot** reference resources in tenant B
- No shared resources, grants, or permissions across tenant boundaries
- Each tenant has its own API key, connection, and reconciliation loop
- System-level operations (Tenant CRUD via `_system/`) use a separate system API key
- GoClaw enforces isolation at the database query level (`WHERE tenant_id = $N`)

To serve multiple tenants, deploy each with its own subdirectory under `--tenants-dir`. There is no mechanism to link or share state between them.

## Org Unit vs Tenant

**Tenant** = first-class resource, API-enforced isolation (GoClaw v1.2.0+).

**Org unit** (engineering, support, data) = logical grouping within a tenant. Represented as:
- Labeled resources within a single manifest
- Separate files in a directory (when using multi-file loading)
- Agent naming conventions (e.g., `engineering-assistant`, `support-bot`)

## Connection Config

Priority: CLI flags > env vars > manifest.

```yaml
apiVersion: gcplane.io/v1
kind: Manifest
metadata:
  name: my-deployment
connection:
  endpoint: https://goclaw.example.com
  token: ${GOCLAW_TOKEN}
  tenantId: ${GOCLAW_TENANT_ID}  # optional — scopes all requests to tenant
resources: []
```

**tenantId** usage:
- Passed as `X-GoClaw-Tenant-Id` header in all HTTP requests
- Passed as `tenant_id` in WebSocket connect handshake
- Not needed when API key is already tenant-bound (auto-scoped by GoClaw)
- Supported via manifest, CLI `--tenant-id`, or `GCPLANE_TENANT_ID` env var

## Multi-Tenant Serve

Run `gcplane serve --tenants-dir` for single HTTP server, multiple tenant reconcile loops:

```bash
gcplane serve --tenants-dir ./tenants/ --interval 30s --addr :8480
```

Each tenant subdirectory must contain at least one YAML with a `connection` block. Subdirectory name becomes tenant identifier in status endpoints.

API endpoints:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/status` | Aggregated status for all tenants |
| GET | `/api/v1/status/{tenant}` | Status for a single tenant |
| POST | `/api/v1/sync` | Trigger sync for all tenants |
| POST | `/api/v1/sync/{tenant}` | Trigger sync for one tenant |
| GET | `/metrics` | Aggregated Prometheus metrics |

Each tenant runs independently — one tenant failure doesn't affect others. `--tenants-dir` mutually exclusive with `-f`/`--file` and `--repo`.
