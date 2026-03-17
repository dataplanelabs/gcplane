# Multi-Tenant Structure

## Overview

Each tenant is an organization/customer with dedicated GoClaw infrastructure (instance + database). Environments (dev/dev/prod) are separate deployments within a tenant.

## Directory Layout

```
goclaw-config/
├── tenants/
│   ├── acme-corp/                      # tenant = org/customer
│   │   ├── base/                       # shared resources across envs
│   │   │   └── providers.yaml
│   │   ├── dev/
│   │   │   ├── connection.yaml         # dev GoClaw endpoint + token
│   │   │   └── manifest.yaml
│   │   └── prod/
│   │       ├── connection.yaml         # prod GoClaw endpoint + token
│   │       └── manifest.yaml
│   │
│   └── globex-inc/
│       ├── base/
│       │   └── providers.yaml
│       ├── dev/
│       │   └── manifest.yaml
│       └── prod/
│           └── manifest.yaml
│
└── platform/                           # platform team infra
    └── manifest.yaml
```

## Isolation Model

| Boundary | Isolation Level |
|----------|----------------|
| **Tenant** | Full — separate GoClaw instance + PostgreSQL database |
| **Environment** | Full — separate deployment (dev/dev/prod) |
| **Org unit** | Logical — resource grouping within a manifest |

## Tenant vs Org Unit

**Tenant** = infrastructure boundary. Each tenant gets:
- Dedicated GoClaw deployment
- Dedicated PostgreSQL database
- Own connection config (endpoint + token)
- Independent scaling and upgrades

**Org unit** (engineering, support, data) = logical grouping within a tenant. NOT separate infrastructure. Represented as:
- Labeled resources within a single manifest
- Separate files in a directory (when using multi-file loading)
- Agent naming conventions (e.g., `engineering-assistant`, `support-bot`)

## Connection Per Tenant

Each tenant/environment has its own connection:

```yaml
# tenants/acme-corp/prod/connection.yaml
apiVersion: gcplane.io/v1
kind: Manifest
metadata:
  name: acme-corp-prod
connection:
  endpoint: https://acme-goclaw.example.com
  token: ${ACME_PROD_GOCLAW_TOKEN}
resources: []
```

## Serve Mode Per Tenant

Run one `gcplane serve` instance per tenant/environment:

```bash
# Acme Corp prod
gcplane serve --repo git@github.com:org/goclaw-config.git \
  --path tenants/acme-corp/prod/manifest.yaml

# Globex Inc dev
gcplane serve --repo git@github.com:org/goclaw-config.git \
  --path tenants/globex-inc/dev/manifest.yaml
```

Or deploy as k8s Deployments — one per tenant/environment.

## Multi-Tenant Serve

`gcplane serve --tenants-dir` discovers all subdirectories under the given path and starts an independent reconcile loop per tenant. All loops share one HTTP server.

```bash
gcplane serve --tenants-dir tenants/ --interval 30s --addr :8480
```

Each subdirectory must contain at least one YAML file with a `connection` block:

```yaml
# tenants/acme-corp/connection.yaml
apiVersion: gcplane.io/v1
kind: Manifest
metadata:
  name: acme-corp
connection:
  endpoint: https://acme-goclaw.example.com
  token: ${ACME_GOCLAW_TOKEN}
resources: []
```

### API endpoints (multi-tenant mode)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/status` | Aggregated status for all tenants |
| GET | `/api/v1/status/{tenant}` | Status for a single tenant |
| POST | `/api/v1/sync` | Trigger sync for all tenants |
| POST | `/api/v1/sync/{tenant}` | Trigger sync for one tenant |
| GET | `/metrics` | Aggregated Prometheus metrics |

### Isolation

- Each tenant runs in its own goroutine — one tenant failure does not affect others.
- Each tenant has its own connection token — no cross-tenant leakage.
- `--tenants-dir` is mutually exclusive with `-f`/`--file` and `--repo`.
