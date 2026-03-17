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

## Future: Multi-Tenant Serve

Currently `gcplane serve` watches one manifest path. Future enhancement: directory-aware mode that discovers and reconciles all tenants:

```bash
gcplane serve --repo git@github.com:org/goclaw-config.git \
  --path tenants/ --mode multi-tenant
```

This would scan `tenants/*/prod/manifest.yaml`, resolve each tenant's connection, and reconcile independently.
