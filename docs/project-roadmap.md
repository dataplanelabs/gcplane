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

### v0.2.x (2026-03-17) — Production Hardening
- Observe fidelity: field exclusion lists (no more *** masking)
- MCP grants management: declarative agent grants
- Export command: dump live GoClaw state as manifest YAML
- Diff command: colorized drift detection
- Server handler tests
- AgentTeam v2 settings (notifications, delivery mode)
- Channel display names + open policies

### v0.3.0 (2026-03-17) — Multi-Tenant
- Multi-tenant serve: --tenants-dir with per-tenant controllers
- Per-tenant HTTP endpoints: /api/v1/status/{tenant}
- Webhook signature verification (GitHub HMAC + GitLab token)
- Tool configuration docs + per-agent overrides
- Directory hashing for FileSource

### v0.4.0 (2026-03-17) — Composites & Operations
- Composite resources: template-based abstractions (ChatBot = Agent + Channel)
- Destroy command: tear down all gcplane-managed resources
- Resource labels: --label filtering on plan/apply
- 24 resources in local-dev example (9 agents, 3 teams, 3 MCP, 2 channels, 3 cron)

## Next: v0.5.0 — Stability & DX

### P1: End-to-End Testing
- Automated e2e test in CI (spin up GoClaw via docker compose)
- Test all commands: validate, plan, apply, diff, export, destroy
- Composite expansion e2e test
- Multi-tenant serve e2e test

### P1: Provider Test Coverage
- Mock HTTP/WS for provider package tests
- Test all 9 observe/create/update/delete methods
- Target: 80%+ overall coverage

### P2: Config File Support
- `gcplane.yaml` or `.gcplane.yaml` as default config (no -f needed)
- Auto-discover manifest in current directory
- Support `GCPLANE_CONFIG` env var

### P2: Init Command
- `gcplane init` — generate starter manifest interactively
- Prompt for provider type, model, agent name
- Generate .env.example with required vars

### P2: Dry-run Destroy
- `gcplane destroy --dry-run` — preview what would be deleted
- Consistent with plan (dry-run) → apply pattern

## Future: v0.6.0 — Enterprise

### Drift Alerting
- Prometheus alerts on drift detection
- Slack/webhook notifications on sync failures
- Configurable alert thresholds

### Audit Log
- Log all apply/prune/destroy operations with timestamps
- Export audit trail as JSON/CSV
- Integration with external logging (stdout structured logs)

### Backup Before Destroy
- Auto-export state before destructive operations
- `gcplane destroy --backup` creates a manifest snapshot
- Rollback via `gcplane apply -f backup.yaml`
