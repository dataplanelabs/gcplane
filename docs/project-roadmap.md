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

### v0.5.0 (2026-03-17) — Stability & Deploy-Anywhere
- Core principle: Deploy-Anywhere, Minimal Dependencies (under 10 deps)
- Test coverage: 41.5% → 81.3% (161 tests, all 9 resource kinds covered)
- Provider package: 10.8% → 81.9% (HTTP + WS mock tests)
- Source package: 16.3% → 86.0% (FileSource dirs + GitSource)
- Controller package: 57.7% → 91.4% (reconcile loop + metrics)
- Drift alerting: Prometheus metrics (gcplane_drift_detected_total, gcplane_drift_resources)
- Slack webhook notifications on drift (stdlib net/http, no SDK)
- Structured logging: log/slog with GCPLANE_LOG_FORMAT=json support
- K8s deployment: kustomize base + dev/staging/prod overlays
- Docker compose for local dev
- Security hardening: runAsNonRoot, readOnlyRootFilesystem, drop ALL caps
- E2E: test-diff, test-composite, test-destroy Makefile targets + CI steps
- Bugfix: hash[:12] panic guard in controller

### v0.6.0 (2026-03-18) — DX & Enterprise
- Enhanced init: `gcplane init` with interactive provider type selection (anthropic, openai, openrouter, custom)
- Config auto-discovery: `gcplane.yaml` or `.gcplane.yaml` as default (no -f needed)
- Audit logging: `--log-file` flag for apply and destroy commands
- Backup before destroy: `gcplane destroy --backup` auto-exports state snapshot
- Version update notifications: auto-check for new releases
- Install script for easy setup

## Next: v0.7.0 — Advanced Features

### P1: Config File Support
- Support `GCPLANE_CONFIG` env var for custom config paths
- Config file validation and schema documentation

### P2: Advanced Audit
- Export audit trail as JSON/CSV
- Integration with external logging systems
- Audit event filtering and search
