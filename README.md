# GCPlane

[![CI](https://github.com/dataplanelabs/gcplane/actions/workflows/ci.yml/badge.svg)](https://github.com/dataplanelabs/gcplane/actions/workflows/ci.yml)
[![Release](https://github.com/dataplanelabs/gcplane/actions/workflows/release.yml/badge.svg)](https://github.com/dataplanelabs/gcplane/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/dataplanelabs/gcplane)](https://goreportcard.com/report/github.com/dataplanelabs/gcplane)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Declarative GitOps control plane for [GoClaw](https://github.com/nextlevelbuilder/goclaw) deployments.

GCPlane manages GoClaw resources — agents, providers, channels, MCP servers, cron jobs, and agent teams — through YAML manifests with a reconcile-and-converge model.

## Features

- **Declarative manifests** — k8s-style YAML with camelCase keys
- **Plan → Apply** — preview changes before applying
- **Serve mode** — continuous reconciliation with health/metrics endpoints
- **Git source** — watch a git repo for manifest changes (GitOps)
- **Prune** — safely delete resources removed from manifest (`--prune`)
- **Reference validation** — catch broken cross-resource references before apply
- **Export** — dump live GoClaw state as manifest YAML
- **Diff** — detect drift between manifest and live state
- **Pluggable providers** — built for GoClaw (and extensible to any xClaw exposed API)
- **Multi-platform** — Linux, macOS, Windows (amd64/arm64)

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  YAML Manifest (camelCase)                                  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐        │
│  │ Provider │ │  Agent   │ │ Channel  │ │ MCPServer│ ...    │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘        │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│  GCPlane Engine                                             │
│                                                             │
│  ┌────────────┐  ┌──────────────┐  ┌──────────────────────┐ │
│  │  Validate  │→ │   Reconcile  │→ │  Apply (Create/      │ │
│  │  (refs +   │  │  (Observe →  │  │   Update/Delete)     │ │
│  │   schema)  │  │   Compare)   │  │                      │ │
│  └────────────┘  └──────────────┘  └──────────────────────┘ │
│                                                             │
│  ┌─────────────────────┐  ┌───────────────────────────────┐ │
│  │  Key Translation    │  │  Source (File / Git repo)     │ │
│  │  camelCase ↔ snake  │  │  SHA256 / commit hash skip    │ │
│  └─────────────────────┘  └───────────────────────────────┘ │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│  GoClaw Instance                                            │
│  HTTP REST API (:18790) + WebSocket RPC v3                  │
│                                                             │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐        │
│  │Providers │ │ Agents   │ │Channels  │ │MCP/Teams │        │
│  │(13+ LLM) │ │(AI bots) │ │(TG/Slack)│ │(tools)   │        │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘        │
│                        ▼                                    │
│                   PostgreSQL                                │
└─────────────────────────────────────────────────────────────┘
```

### Serve Mode (Continuous Reconciliation)

```
┌──────────┐     ┌────────────┐     ┌──────────────┐
│ Git Repo │────▶│ Controller │────▶│ GoClaw API   │
│ or File  │     │ (30s loop) │     │              │
└──────────┘     └─────┬──────┘     └──────────────┘
                       │
                 ┌─────▼──────┐
                 │ HTTP Server│
                 │ :8480      │
                 │            │
                 │ /healthz   │
                 │ /readyz    │
                 │ /metrics   │
                 │ /status    │
                 │ /sync      │
                 └────────────┘
```

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
| `diff` | Show drift between manifest and live state |
| `export` | Dump live GoClaw state as manifest YAML |
| `serve` | Continuous reconciliation (GitOps mode) |

### Key Flags

| Flag | Description |
|------|-------------|
| `-f, --file` | Manifest file or directory |
| `--prune` | Delete resources removed from manifest |
| `--auto-approve` | Skip confirmation prompt |
| `--repo` | Git repository URL (serve mode) |
| `--interval` | Reconciliation interval (default: 30s) |

## Resource Kinds

| Kind | Description |
|------|-------------|
| `Provider` | LLM provider (Anthropic, OpenAI, Gemini, etc.) |
| `Agent` | AI agent with model, tools, and identity |
| `Channel` | Messaging channel (Telegram, Slack, Discord) |
| `MCPServer` | MCP tool server with agent grants |
| `CronJob` | Scheduled task |
| `AgentTeam` | Agent team (v1/v2 with notifications) |
| `Tool` | Custom tool definition |
| `Skill` | Agent skill (update only) |
| `TTSConfig` | Text-to-speech settings |

## Documentation

| Document | Description |
|----------|-------------|
| [Manifest Reference](docs/manifest-reference.md) | Resource kinds, secrets, prune, serve endpoints |
| [System Architecture](docs/system-architecture.md) | Package structure, data flow, design decisions |
| [Tenant Structure](docs/tenant-structure.md) | Multi-tenant deployment patterns |
| [Usage Guide](docs/usage-guide.md) | Detailed command usage and examples |
| [Project Roadmap](docs/project-roadmap.md) | Release plan and future features |
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
