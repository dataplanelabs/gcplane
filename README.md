# GCPlane

GitOps-style control plane for managing [GoClaw](https://github.com/dataplanelabs/goclaw) deployments.

GCPlane reads declarative YAML manifests describing your desired GoClaw configuration (agents, providers, channels, cron jobs, MCP servers, etc.) and reconciles them against the actual state via GoClaw's API.

## Install

```bash
go install github.com/dataplanelabs/gcplane@latest
```

## Quick Start

```yaml
# gcplane.yaml
apiVersion: gcplane.io/v1
kind: Manifest

metadata:
  name: my-deployment

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

```bash
# Validate manifest
gcplane validate -f gcplane.yaml

# Preview changes (dry-run)
gcplane plan -f gcplane.yaml

# Apply changes
gcplane apply -f gcplane.yaml
```

## Commands

| Command    | Description                                   |
|------------|-----------------------------------------------|
| `validate` | Validate manifest schema (no connection needed) |
| `plan`     | Show changes required to reach desired state    |
| `apply`    | Apply manifest with confirmation prompt         |
| `version`  | Print version                                   |

## Connection Config

Priority: CLI flags > environment variables > manifest `connection` block.

| Flag         | Env Var            | Description        |
|--------------|--------------------|--------------------|
| `--endpoint` | `GCPLANE_ENDPOINT` | GoClaw endpoint URL |
| `--token`    | `GCPLANE_TOKEN`    | GoClaw auth token   |
| `-f, --file` | —                  | Manifest file/dir   |
| `-v`         | —                  | Verbose output      |

## Supported Resources

| Kind              | Transport | Operations     |
|-------------------|-----------|----------------|
| `Provider`        | HTTP      | create, update |
| `Agent`           | HTTP      | create, update |
| `ChannelInstance`  | HTTP      | create, update |
| `MCPServer`       | HTTP      | create, update |
| `Skill`           | HTTP      | update only    |
| `CustomTool`      | HTTP      | create, update |
| `CronJob`         | WebSocket | create, update |
| `Team`            | WebSocket | create, update |
| `TTSConfig`       | WebSocket | create, update |

## Secret Resolution

```yaml
token: ${ENV_VAR}           # environment variable
apiKey: file:///path/to/key  # file reference
```

## Plan Output

```
GCPlane Plan: 1 to create, 1 to update, 0 unchanged

+ Provider/anthropic
~ Agent/assistant
    model: "claude-haiku-4-5-20251001" → "claude-sonnet-4-20250514"
```

## Docs

- [System Architecture](docs/system-architecture.md)
- [Code Standards](docs/code-standards.md)
- [Usage Guide](docs/usage-guide.md)
- [Examples](examples/)

## License

MIT
