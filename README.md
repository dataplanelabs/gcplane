# GCPlane

[![CI](https://github.com/dataplanelabs/gcplane/actions/workflows/ci.yml/badge.svg)](https://github.com/dataplanelabs/gcplane/actions/workflows/ci.yml)
[![Release](https://github.com/dataplanelabs/gcplane/actions/workflows/release.yml/badge.svg)](https://github.com/dataplanelabs/gcplane/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/dataplanelabs/gcplane)](https://goreportcard.com/report/github.com/dataplanelabs/gcplane)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Declarative GitOps control plane for [GoClaw](https://github.com/dataplanelabs/goclaw) deployments.

GCPlane manages GoClaw resources (agents, providers, channels, MCP servers, cron jobs, teams) through YAML manifests — like Terraform for your AI gateway.

## Features

- **Declarative manifests** — k8s-style YAML with camelCase keys
- **Plan → Apply** — preview changes before applying (like `terraform plan`)
- **Serve mode** — continuous reconciliation with health/metrics endpoints
- **Git source** — watch a git repo for manifest changes (GitOps)
- **Prune** — safely delete resources removed from manifest (`--prune`)
- **Reference validation** — catch broken cross-resource references before apply
- **Pluggable providers** — built for GoClaw (and extensible to any xClaw exposed API)
- **Multi-platform** — Linux, macOS, Windows (amd64/arm64)

## Quick Start

```bash
# Install
go install github.com/dataplanelabs/gcplane@latest

# Create manifest
cat > manifest.yaml << 'EOF'
apiVersion: gcplane.io/v1
kind: Manifest
metadata:
  name: my-setup
connection:
  endpoint: http://localhost:18790
  token: ${GOCLAW_TOKEN}
resources:
  - kind: Provider
    name: anthropic
    spec:
      displayName: "Anthropic"
      providerType: anthropic_native
      apiBase: https://api.anthropic.com
      apiKey: ${ANTHROPIC_API_KEY}
      enabled: true
  - kind: Agent
    name: assistant
    spec:
      displayName: "Assistant"
      provider: anthropic
      model: claude-sonnet-4-20250514
      agentType: open
      status: active
      isDefault: true
EOF

# Preview → Apply
gcplane plan -f manifest.yaml
gcplane apply -f manifest.yaml
```

## Commands

| Command | Description |
|---------|-------------|
| `validate` | Check manifest syntax + references (offline) |
| `plan` | Preview changes (dry-run) |
| `apply` | Apply manifest to GoClaw |
| `serve` | Continuous reconciliation (GitOps mode) |

### Flags

| Flag | Description |
|------|-------------|
| `-f, --file` | Manifest file path |
| `--prune` | Delete resources removed from manifest |
| `--auto-approve` | Skip confirmation prompt |
| `--repo` | Git repository URL (serve mode) |
| `--interval` | Reconciliation interval (default: 30s) |

## Documentation

| Document | Description |
|----------|-------------|
| [Manifest Reference](docs/manifest-reference.md) | Resource kinds, secret resolution, serve endpoints |
| [System Architecture](docs/system-architecture.md) | Package structure, data flow, design decisions |
| [Tenant Structure](docs/tenant-structure.md) | Multi-tenant deployment patterns |
| [Usage Guide](docs/usage-guide.md) | Detailed command usage and examples |
| [Code Standards](docs/code-standards.md) | Development conventions |

## Development

```bash
# Setup (requires Docker for GoClaw)
cp .env.example .env   # fill in credentials
make setup              # start GoClaw + apply config

# Install git hooks
git config core.hooksPath .githooks

# Test
make test               # unit tests
make test-e2e           # full e2e (reset + plan + apply + serve)

# Serve (continuous reconciliation)
make serve
```

## License

MIT
