# GCPlane Code Standards

## Core Principles
- **Deploy-Anywhere**: Single binary deploys to local dev, VPS (systemd/docker), and k8s equally. No platform-specific abstractions.
- **Minimal Dependencies**: Keep total deps under 10. Prefer stdlib. No heavy frameworks, no SDKs for simple HTTP calls.
- **Self-Contained**: Binary includes all logic. Config via env vars + YAML manifest. No external config services.

## Language & Tooling
- **Go 1.25+** (pure Go, no CGO dependencies)
- **Cobra 1.10.2** for CLI commands
- **YAML v3 3.0.1** for manifest parsing
- **gorilla/websocket 1.5.3** for WS RPC
- **tview 0.42.0** for terminal UI
- **tcell/v2 2.8.1** for terminal rendering

## Version Management

### Build-Time Versioning
GCPlane version set at build time via `-ldflags`:
```bash
go build -ldflags="-X github.com/dataplanelabs/gcplane/cmd.Version=$(git describe --tags)" -o gcplane .
```

Default version (when unset): `"dev"`

### Version Checking
- **Update Checker**: Queries GitHub API (internal/update/update.go)
- **Cache TTL**: 24 hours
- **Trigger**: On every command start (background goroutine)
- **User Notification**: Printed to stderr if newer version available

### Semantic Versioning
GCPlane follows semver:
- **Major**: Breaking API changes, significant feature rewrites
- **Minor**: New features, non-breaking enhancements
- **Patch**: Bug fixes, security patches

Examples:
- v0.7.0 → v0.7.2: Patch (credentials restructure, bug fixes)
- v0.7.2 → v0.8.0: Minor (new Tenant resource kind, multi-tenant mode)
- v0.8.0 → v1.0.0: Major (production release, stable API, removed Tool/TTSConfig, added SystemConfig)

## File Organization
- `cmd/` — CLI commands, one file per command
- `internal/` — private packages, organized by domain
- Each resource type gets its own file in `provider/goclaw/` (e.g., `secure_cli.go`, `secure_cli_grants.go`)
- Keep files under 200 lines where practical

## Naming Conventions
- **Files**: snake_case (Go convention)
- **Packages**: lowercase, single word
- **Types/Functions**: PascalCase (exported), camelCase (unexported)
- **Resource keys**: kebab-case in manifests

## Error Handling
- Sentinel errors for HTTP status classification (`ErrNotFound`, `ErrUnauthorized`)
- Wrap errors with context: `fmt.Errorf("action %s: %w", key, err)`
- Provider methods return `nil, nil` for "resource not found" (not an error)

## Provider Pattern
Each resource implements 3 methods on `*Provider`:
```go
func (p *Provider) observeX(key string) (map[string]any, error)  // nil = not found
func (p *Provider) createX(key string, spec map[string]any) error
func (p *Provider) updateX(key string, spec map[string]any) error
```

Routing in `provider.go` dispatches by `ResourceKind`.

**Composite Keys**: Some resources use composite natural keys (e.g., SecureCLIGrant uses `binaryName--agentKey`). The observe method parses the composite key and validates both parts resolve to valid resource UUIDs before comparison.

## Provider Options
Use the Option pattern for Provider configuration:
```go
// Create provider with options
provider := goclaw.New(
  goclaw.WithTenantID("acme-corp"),
  goclaw.WithHTTPClient(client),
)
```

Options are variadic functions that modify Provider state, enabling flexible, composable configuration without constructor parameters.

## Testing
- Unit tests in `*_test.go` alongside source files
- Use `testing.T` and table-driven tests
- Mock the `ProviderInterface` for reconciler tests
- Use `t.TempDir()` for file-based tests (auto-cleanup)
- No external services required for unit tests

## Secret Handling
- Never log or display resolved secrets
- API keys masked as `"***"` in GoClaw responses — skip in comparison
- Support `${ENV_VAR}` and `file://path` patterns

## Tenant Isolation Pattern

In multi-tenant mode (v0.8.0+), enforce isolation via:

**1. Observation filtering** — Apply `matchesTenant()` to all observe/listAll results:
```go
if strVal(inst, "name") == key && p.matchesTenant(inst) {
    return translateResult(stripInternal(inst)), nil
}
```

**2. Creation injection** — Resolve tenant slug to UUID, inject into POST body:
```go
if p.tenantID != "" {
    uuid, err := p.resolveTenantUUID()
    if err != nil {
        return err
    }
    spec["tenantId"] = uuid
}
```

**3. Resource filtering** — `matchesTenant()` uses cached tenant UUID resolution:
- Single-tenant mode (no tenant ID): always true
- Multi-tenant: filters by `tenant_id` field in response
- Header-based scoping fallback if field absent

**4. HTTP headers** — Tenant headers on all requests:
```go
req.Header.Set("X-GoClaw-Tenant-Id", p.tenantID)
req.Header.Set("X-GoClaw-User-Id", p.userID)
```

**5. WebSocket handshake** — Tenant ID in connect message:
```json
{"token": "...", "user_id": "...", "tenant_id": "..."}
```

**Examples**:
- Acme Corp agents: `examples/local-dev-mt/acme-corp/agents.yaml` (tenant: acme-corp)
- Startup IO agents: `examples/local-dev-mt/startup-io/agents.yaml` (tenant: startup-io)

**Testing**: 12 tenant isolation tests in internal/provider/goclaw/tenant_test.go
