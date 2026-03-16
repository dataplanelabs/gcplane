---
phase: 3
priority: critical
effort: M
status: pending
---

# Phase 3: Provider — HTTP Resources

## Overview

Implement Observe/Create/Update for resources with HTTP REST endpoints: Provider, Agent, ChannelInstance, MCPServer, Skill, CustomTool.

## Related Code Files

- **Modify:** `internal/provider/goclaw/provider.go` — update method routing
- **Create:** `internal/provider/goclaw/agents.go` — agent CRUD
- **Create:** `internal/provider/goclaw/providers.go` — provider CRUD
- **Create:** `internal/provider/goclaw/channels.go` — channel instance CRUD
- **Create:** `internal/provider/goclaw/mcp_servers.go` — MCP server CRUD
- **Create:** `internal/provider/goclaw/skills.go` — skill observe/update
- **Create:** `internal/provider/goclaw/custom_tools.go` — custom tool CRUD

## Key Insights

### Identity Matching (Natural Key → UUID)

| Resource | Natural Key | List Endpoint | Match Field |
|----------|-------------|---------------|-------------|
| Agent | `agent_key` | `GET /v1/agents` | `agentKey` in response |
| Provider | `name` | `GET /v1/providers` | `name` in response |
| ChannelInstance | `name` | `GET /v1/channels/instances` | `name` in response |
| MCPServer | `key` | `GET /v1/mcp/servers` | `key` in response |
| Skill | `key` | `GET /v1/skills` | `key` in response |
| CustomTool | `name` | `GET /v1/tools/custom` | `name` in response |

**Pattern**: All observe methods do `GET /v1/<resource>` → list → filter by natural key → return match or nil.

### API Key Masking
- Providers return `apiKey: "***"` — never compare this field
- On update, skip apiKey if manifest value resolves to same as current (can't compare)
- Always send apiKey on create

## Implementation Steps

### Per resource file, implement 3 functions:

```go
func (p *Provider) observeAgent(key string) (map[string]any, error)
func (p *Provider) createAgent(key string, spec map[string]any) error
func (p *Provider) updateAgent(key string, spec map[string]any) error
```

### agents.go
1. `observeAgent`: GET /v1/agents → find by agentKey match → return spec-like map
2. `createAgent`: POST /v1/agents with {agentKey, displayName, provider, model, ...}
3. `updateAgent`: PUT /v1/agents/{id} with spec fields

### providers.go
1. `observeProvider`: GET /v1/providers → find by name → return (mask apiKey)
2. `createProvider`: POST /v1/providers with {name, displayName, baseUrl, apiKey, ...}
3. `updateProvider`: PUT /v1/providers/{id} — always send apiKey from manifest

### channels.go
1. `observeChannelInstance`: GET /v1/channels/instances → find by name
2. `createChannelInstance`: POST /v1/channels/instances
3. `updateChannelInstance`: PUT /v1/channels/instances/{id}

### mcp_servers.go, skills.go, custom_tools.go
Same pattern. Skills may be observe+update only (no create via HTTP if skill auto-discovered).

## Todo

- [ ] agents.go — observe/create/update
- [ ] providers.go — observe/create/update (handle API key masking)
- [ ] channels.go — observe/create/update
- [ ] mcp_servers.go — observe/create/update
- [ ] skills.go — observe/update
- [ ] custom_tools.go — observe/create/update
- [ ] Update provider.go routing to call new functions
- [ ] Compile check

## Success Criteria

- Can observe all 6 HTTP resource types from GoClaw
- Can create/update resources via HTTP
- Natural key → UUID resolution works via list+filter
- API key masking handled correctly for providers
