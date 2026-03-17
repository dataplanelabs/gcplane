# Project Roadmap

## Released

### v0.1.0 (2026-03-17) — Foundation
- Declarative YAML manifests (k8s-style camelCase)
- 9 resource types with full CRUD + idempotency
- Commands: validate, plan, apply, serve
- Continuous reconciliation (serve mode)
- File + git source watching
- Pre-flight reference validation
- Prune support (--prune)
- Directory loading
- CI/CD + multi-platform release

## Next: v0.2.0 — Production Hardening

### P1: Observe Fidelity
- Replace `"***"` masking with proper field-level compare exclusions
- Per-resource field mapping (manifest schema → API response schema)
- Detect real drift for write-only fields (agentKey, botToken, grants)

### P1: MCP Grants Management
- Separate API calls for agent grants (POST /v1/mcp/agents-grants)
- Grant/revoke agents to MCP servers declaratively
- Idempotent grant comparison

### P1: Test Coverage
- Provider package tests (mock HTTP/WS)
- Server handler tests (httptest)
- Controller integration tests
- Target: 80%+ coverage

### P2: Webhook Trigger
- GitHub webhook signature verification (HMAC-SHA256)
- GitLab webhook support
- Configurable webhook secret

### P2: Export Command
- `gcplane export` — dump current GoClaw state as manifest YAML
- Bootstrap existing deployments into gcplane management

### P2: Diff Command
- `gcplane diff` — show drift between manifest and live state
- Colorized output with field-level comparison

## Future: v0.3.0 — Multi-Tenant

### Multi-Tenant Serve
- One gcplane instance watches multiple tenant directories
- Per-tenant connection config
- Tenant-scoped metrics and status

### Composite Resources
- Define abstractions (ChatBot = Provider + Agent + Channel)
- Template expansion before reconciliation
- Reduce manifest boilerplate for repeated patterns

### Built-in Tool Config
- Manage GoClaw built-in tool settings (exec, web_fetch, etc.)
- Per-agent tool policy overrides

## Future: v0.4.0 — Enterprise

### RBAC / Approval Workflow
- Require approval for destructive operations
- Audit log of all apply/prune operations
- Integration with external approval systems

### Drift Alerting
- Prometheus alerts on drift detection
- Slack/webhook notifications on sync failures

### Backup / Restore
- Export full GoClaw state before destructive operations
- Rollback to previous state on failure
