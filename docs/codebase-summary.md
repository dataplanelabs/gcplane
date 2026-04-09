# GCPlane Codebase Summary

Generated: 2026-04-06 | Based on repomix analysis and v1.2.0 release

## Overview

GCPlane v1.2.0 is a declarative GitOps control plane for managing GoClaw deployments. Single binary, under 10 deps, pure Go 1.25. Total codebase: ~20,600 LOC across cmd/ and internal/ packages with 183+ tests (81.3%+ coverage).

## Quick Stats

| Metric | Value |
|--------|-------|
| **Language** | Go 1.25+ |
| **Total Dependencies** | 5 direct |
| **Test Coverage** | 81.3%+ (172+ tests, +11 TUI in v1.2) |
| **Codebase Lines** | ~20,600 LOC (v1.2.0) |
| **Binary Size** | Single statically-linked executable |
| **Resource Kinds** | 14 (Tenant + 8 core + 5 config) |
| **Transport Protocols** | HTTP REST + WebSocket RPC v3 |

## Technology Stack

### Core Dependencies
- **gorilla/websocket** v1.5.3 — WebSocket RPC protocol
- **cobra** v1.10.2 — CLI framework
- **gopkg.in/yaml.v3** v3.0.1 — YAML parsing
- **tview** v0.42.0 — Terminal UI
- **tcell/v2** v2.8.1 — Terminal rendering

### Build & Deployment
- **Go 1.25.8** minimum (no CGO required)
- **Docker** for containerization
- **Kustomize** for k8s deployments (base + 3 overlays: dev, staging, prod)
- **GitHub Actions** for CI/CD (golangci-lint-action v7, release workflows)

## Project Structure

### cmd/ — CLI Commands (13 files, ~1372 LOC)
| Command | Purpose | Status |
|---------|---------|--------|
| `root.go` | Version, global flags, update checker | v0.6.0+ |
| `config.go` | Connection resolution (flags > env > manifest) | v0.1.0+ |
| `validate.go` | Schema validation (no GoClaw connection) | v0.1.0+ |
| `plan.go` | Dry-run reconciliation + --prune detection | v0.1.0+ |
| `apply.go` | Execute mutations with confirmation | v0.1.0+ |
| `serve.go` | Continuous reconciliation (file/git sources) | v0.1.0+ |
| `top.go` | Interactive k9s-style TUI dashboard | v0.7.0+ |
| `diff.go` | Drift detection (compare vs live) | v0.2.0+ |
| `export.go` | Export GoClaw state as manifest YAML | v0.2.0+ |
| `destroy.go` | Remove all gcplane-managed resources | v0.4.0+ |

### internal/manifest/ — YAML Manifest Handling (~1330 LOC)
**Purpose**: Load, parse, validate, and compose YAML manifests

| File | Responsibility |
|------|-----------------|
| `types.go` | Manifest, Resource, ResourceKind, Metadata structures |
| `loader.go` | File/directory loading, merging, secret resolution |
| `validate.go` | Schema validation rules for all resource kinds |
| `field_config.go` | WriteOnlyFields, ResourceDefaults per kind |
| `composite.go` | CompositeDefinition expansion via text/template |
| `delete_order.go` | Reverse dependency order for safe prune |
| `filter.go` | Label-based resource filtering |
| `validate_refs.go` | Reference validation (provider → agent, etc.) |

### internal/provider/goclaw/ — GoClaw API Client (~4852 LOC)
**Purpose**: HTTP REST + WebSocket RPC communication with GoClaw

**Architecture**:
- `provider.go` — Provider struct, routing dispatcher, Option pattern
- `http_client.go` — Authenticated HTTP client with tenant headers
- `ws_client.go` — WebSocket RPC v3 handshake, tenant_id support
- `helpers.go` — stripInternal, strVal, translateResult utilities

**Resource Implementations** (per-file):
- `agents.go` — Agent CRUD (HTTP, deletable)
- `channels.go` — Channel CRUD (HTTP, credentials nested)
- `providers.go` — Provider CRUD (HTTP, API key masking)
- `mcp_servers.go` — MCPServer CRUD (HTTP)
- `skills.go` — Skill observe/update only (HTTP, not deletable)
- `tenants.go` — Tenant CRUD (system scope only)
- `builtin_tool_configs.go` — Per-tenant builtin tool config
- `skill_configs.go` — Per-tenant skill enable/disable
- `mcp_credentials.go` — Per-user MCP server credentials
- `system_configs.go` — Per-tenant system settings (v1.0.0+)
- `secure_cli.go` — SecureCLI CRUD (HTTP, deletable, binary_name key) (NEW in v1.1.0)
- `secure_cli_grants.go` — SecureCLIGrant CRUD (HTTP, deletable, composite key) (NEW in v1.1.0)
- `cron_jobs.go` — CronJob CRUD (WebSocket, deletable, agent_key resolution)
- `teams.go` — AgentTeam CRUD (WebSocket, deletable)

### internal/reconciler/ — Observe→Compare→Act Engine (~947 LOC)
**Purpose**: Reconciliation logic for dry-run and apply modes

| File | Responsibility |
|------|-----------------|
| `engine.go` | Main reconciliation loop, prune detection, secret resolution |
| `compare.go` | Deep spec comparison, field-level diffs, masked field skipping |
| `types.go` | Plan, Change, FieldDiff, ApplyResult structures |

### internal/controller/ — Reconciliation Loop (~1438 LOC)
**Purpose**: Continuous reconciliation server for serve mode

| File | Responsibility |
|------|-----------------|
| `controller.go` | Main loop with interval, graceful shutdown |
| `status.go` | k8s-style status conditions (Synced/Error/Drifted) |
| `tenant_manager.go` | Multi-tenant mode: per-tenant controllers |
| `metrics.go` | Prometheus metrics export |

### internal/source/ — Manifest Source Abstraction (~768 LOC)
**Purpose**: File and Git manifest sources with change detection

| File | Responsibility |
|------|-----------------|
| `source.go` | Source interface (GetManifest, HasChanged) |
| `file_source.go` | File watching with SHA256 change detection |
| `git_source.go` | Git clone/fetch with branch checkout, SHA detection |

### internal/server/ — HTTP Server (~519 LOC)
**Purpose**: Health, metrics, status, and webhook endpoints for serve mode

| File | Responsibility |
|------|-----------------|
| `server.go` | Server startup, graceful shutdown, middleware |
| `handlers.go` | /healthz, /readyz, /metrics, /api/v1/status, /api/v1/sync, /api/v1/webhook/git |

### internal/tui/ — Interactive Terminal UI (v1.2.0+, ~5,754 LOC)
**Purpose**: k9s-style resource browser with 3-tab layout, extensible view architecture, and real-time LLM trace capture

**Core Components** (3,467 LOC):
| File | Responsibility |
|------|-----------------|
| `app.go` | Main orchestrator, wires layout/views/keybindings, manages refresh loops (State 10s, Traces 3s, Logs RT) |
| `registry.go` | ViewRegistry: manage tab pages with overlay support (confirm modal, help, span detail) |
| `event_bus.go` | Typed pub/sub with QueueUpdateDraw thread safety |
| `model.go` | Thread-safe shared state with RWMutex for resource filtering |
| `keybindings.go` | Vim-style input dispatcher with modal support, two-key sequences (gg, yy) |
| `actions.go` | Resource operations (apply, delete, edit with $EDITOR) |
| `attach.go` | HTTP polling client for remote gcplane serve instances |
| `trace_store.go` | LLM trace data cache with atomic.Bool fetch gating |
| `live_store.go` | Ring-buffered WS event stream (500-entry capacity) |

**Views Layer** (2,287 LOC):
| File | Responsibility |
|------|-----------------|
| `views/view.go` | View interface (Name, Primitive, Activate) |
| `views/resource_table.go` | State tab: resource list with status coloring (InSync/Drifted/Missing/Error/Extra) |
| `views/resource_detail.go` | YAML view with syntax highlighting |
| `views/drift_view.go` | Field-level drift comparison (red/green diff) |
| `views/trace_list.go` | Traces tab: list of traces (left panel, 2:3 ratio) |
| `views/span_tree.go` | Traces tab: hierarchical span tree (right panel, 1:3 ratio) |
| `views/span_detail.go` | Overlay: detailed trace span information |
| `views/logs_panel.go` | Logs tab: live log table with level filtering |
| `views/confirm_modal.go` | Confirmation dialog for destructive ops |
| `views/help_view.go` | Help overlay with keybinding reference |
| `trace/ring_handler.go` | Custom slog.Handler with 1000-entry ring buffer for API call tracing |
| `trace/entry.go` | TraceEntry structure |
| `trace/types.go` | Event types (EventTraceEntry, etc.) |

### Other Packages

#### internal/display/ (~284 LOC)
- `plan.go` — Terraform-style colored output (+ create, ~ update, - delete)

#### internal/keyconv/ (~271 LOC)
- `keyconv.go` — Bidirectional camelCase ↔ snake_case translation

#### internal/secrets/ (~129 LOC)
- `resolver.go` — ${ENV_VAR}, file://, SOPS support

#### internal/notifier/ (~359 LOC)
- `notifier.go` — Webhook notifications on drift
- `formats.go` — Slack, Discord, Google Chat, Teams, Telegram payloads

#### internal/update/ (~318 LOC)
- `update.go` — Self-update checker via GitHub Releases

#### internal/tui/ (~5,754 LOC)
- 21 files for k9s-style TUI with 3-tab layout (State/Traces/Logs), LLM trace capture, and remote attach mode

### examples/ — Reference Manifests
- **minimal.yaml** — Bare minimum (1 provider, 1 agent)
- **production.yaml** — Production checklist (multi-provider, security)
- **local-dev/** — Full-featured example (9 agents, 3 teams, 3 MCP, 2 channels, 3 crons)
- **local-dev-mt/** — Multi-tenant example (_system/, acme-corp/, startup-io/)
- **composite-example.yaml** — ChatBot composite (Agent + Channel template)
- **tenant-structure/** — Hierarchical tenant organization (dev/prod overlays)

### deploy/ — Kubernetes Manifests (kustomize)
- **base/** — Deployment, Service, ConfigMap, ServiceAccount
- **overlays/dev/** — Dev environment (1 replica, lower resources)
- **overlays/staging/** — Staging environment
- **overlays/prod/** — Production (2 replicas, higher resources)

### .github/workflows/ — CI/CD Automation
- **ci.yml** — Lint, test, build on push
- **e2e.yml** — E2E tests against GoClaw 1.2.0 (reset + apply + destroy)
- **release.yml** — Build + publish multi-platform releases (GitHub, Docker Hub, ghcr.io)
- **upstream-check.yml** — Weekly check for GoClaw updates

## Resource Kinds (14 total)

### System Scope (1)
| Kind | Create | Update | Delete | Transport | Notes |
|------|--------|--------|--------|-----------|-------|
| `Tenant` | ✓ | ✓ | ✓ | HTTP | v0.8.0+, system scope only |

### Core Resources (8)
| Kind | Create | Update | Delete | Transport | Notes |
|------|--------|--------|--------|-----------|-------|
| `Provider` | ✓ | ✓ | ✓ | HTTP | API keys masked as *** |
| `Agent` | ✓ | ✓ | ✓ | HTTP | systemPrompt write-only |
| `Channel` | ✓ | ✓ | ✓ | HTTP | Credentials nested in object |
| `MCPServer` | ✓ | ✓ | ✓ | HTTP | — |
| `Skill` | — | ✓ | — | HTTP | Auto-discovered, not deletable |
| `CronJob` | ✓ | ✓ | ✓ | WebSocket | agent_key → agent_id resolved |
| `AgentTeam` | ✓ | ✓ | ✓ | WebSocket | v2 settings (notifications, delivery mode) |

### Config Resources (5)
| Kind | Create | Update | Delete | Transport | Notes |
|------|--------|--------|--------|-----------|-------|
| `BuiltinToolConfig` | ✓ | ✓ | ✓ | HTTP | Per-tenant, v0.8.0+ |
| `SkillConfig` | ✓ | ✓ | ✓ | HTTP | Per-tenant, v0.8.0+ |
| `MCPCredentials` | ✓ | ✓ | ✓ | HTTP | Per-user, v0.8.0+ |
| `SystemConfig` | ✓ | ✓ | ✓ | HTTP | Per-tenant system settings, v1.0.0+ |
| `SecureCLI` | ✓ | ✓ | ✓ | HTTP | Secure CLI binary configs, v1.1.0+ |

### Child Resources (1)
| Kind | Create | Update | Delete | Transport | Notes |
|------|--------|--------|--------|-----------|-------|
| `SecureCLIGrant` | ✓ | ✓ | ✓ | HTTP | Per-agent CLI overrides, v1.1.0+, non-enumerable |

### Dependency Order
```
Tenant → Provider → Agent → Skill → BuiltinToolConfig → SkillConfig
  → SystemConfig → MCPServer → MCPCredentials → Channel → CronJob → SecureCLI → SecureCLIGrant → AgentTeam
```

Prune deletes in reverse order.

## Key Architectural Patterns

### 1. Provider Option Pattern
Flexible, composable configuration without constructor parameters:
```go
provider := goclaw.New(ep, tok,
  goclaw.WithTenantID("acme-corp"),
  goclaw.WithHTTPClient(customClient),
)
```

### 2. Observe→Compare→Act
Standard GitOps reconciliation flow:
1. **Observe**: Query GoClaw state via HTTP/WS
2. **Compare**: Deep diff (skip masked/write-only fields)
3. **Act**: Create/update/delete in dependency order

### 3. Tenant Isolation Pattern
Multi-tenant safety via:
- **Observation filtering**: `matchesTenant()` on all listAll results
- **Creation injection**: Tenant UUID resolved and injected
- **Resource filtering**: Only tenant-scoped resources matched

### 4. camelCase Manifest Convention
Kubernetes-style camelCase in manifests; internal `keyconv` translates to snake_case for GoClaw API.

### 5. Prune Safety
- Opt-in via `--prune` flag or manifest `prune: true`
- Only deletes `gcplane.io/managed: true` resources
- Deletes in reverse dependency order
- Continue-on-error per resource

### 6. Secret Handling
- Support `${ENV_VAR}` and `file://path` patterns
- API keys masked as `"***"` — skip in comparison
- Resolved at reconciliation time, not load

### 7. No Local State
GoClaw API is single source of truth. GCPlane carries no local state (SQLite removed v0.5.0+).

### 8. Natural Key Resolution
GoClaw uses UUIDs internally. GCPlane manifests use human-readable names. Resolution: list → filter by name → extract UUID.

## Testing

### Test Coverage
- **Overall**: 81.3%+ (183+ tests)
- **Provider**: 81.9% (HTTP + WS mock tests)
- **Source**: 86.0% (FileSource dirs + GitSource)
- **Controller**: 91.4% (reconcile loop + metrics)
- **Reconciler**: High coverage via table-driven tests
- **TUI**: 27 tests (ResourceTable, TraceStore, Registry, RingHandler, LiveStore, smoke test)

### Test Organization
- Unit tests alongside source files (`*_test.go`)
- Table-driven test patterns for complex logic
- Mock `ProviderInterface` for reconciler tests
- `t.TempDir()` for file-based tests (auto-cleanup)
- No external services required for unit tests

### E2E Testing
- **scripts/test-e2e.sh** — Reset GoClaw + apply/plan/destroy + verify
- **CI/CD pipeline** — Runs against GoClaw 1.2.0 on every PR
- Test commands: `mise run test` (unit), `mise run test:e2e` (integration)

## Version & Compatibility

### GCPlane Versions
| Version | Release Date | Major Features |
|---------|-------------|----------------|
| v0.1.0 | 2026-03-17 | Foundation: YAML manifests, 9 kinds, validate/plan/apply/serve |
| v0.2.x | 2026-03-17 | Diff, Export, MCP grants, Channel display names |
| v0.3.0 | 2026-03-17 | Multi-tenant serve, per-tenant endpoints, webhook verification |
| v0.4.0 | 2026-03-17 | Composites, destroy, label filtering |
| v0.5.0 | 2026-03-17 | Stability, 81% test coverage, security hardening, k8s deploy |
| v0.6.0 | 2026-03-18 | Enhanced init, auto-discovery, audit logging, version updates |
| v0.7.0 | 2026-03-18 | Interactive TUI (top command), vim keybindings |
| v0.7.2 | 2026-03-25 | Channel credentials restructure, tenant isolation enforcement |
| v0.8.0 | 2026-03-24 | First-class multi-tenant: Tenant CRUD, per-tenant config, 13 kinds |
| v1.0.0 | 2026-04-02 | Production release: Stable API, removed Tool/TTSConfig, added SystemConfig, 12 kinds |
| v1.1.0 | 2026-04-05 | Secure CLI & Grants: SecureCLI/SecureCLIGrant resources, 14 kinds |
| **v1.2.0** | **2026-04-05** | **TUI Extensibility: View interface, ViewRegistry, EventBus, Trace view with slog capture** |

### GoClaw Compatibility
Tested against: **ghcr.io/nextlevelbuilder/goclaw:full** (v3.x)

**RPC Protocol**: v3 (WebSocket)
- Connect handshake: token, user_id, optional tenant_id
- Tenant headers: X-GoClaw-Tenant-Id, X-GoClaw-User-Id

### Channel Credentials Structure (v0.7.2+)
Moved from top-level to nested `credentials` object:
- **Telegram**: `credentials.token` (single token)
- **Slack**: `credentials.botToken` + `credentials.appToken`
- Other types follow similar patterns

### Write-Only Fields
Excluded from comparison to prevent false diffs:
- Agent: systemPrompt
- Provider: (varies by provider type)
- Channel: credentials object contents
- SecureCLI: env (encrypted environment variables)
- SecureCLIGrant: agentKey, binaryName (manifest references resolved to UUIDs)

## Development Workflow

### Local Setup
```bash
cp .env.example .env  # Fill in GOCLAW_TOKEN, ANTHROPIC_API_KEY, etc.
mise run setup         # Start GoClaw docker + apply examples
```

### Testing
```bash
mise run test          # Unit tests
mise run test:e2e      # E2E tests (reset + apply + destroy)
mise run reset         # Wipe GoClaw + re-apply manifest
mise run serve         # Continuous reconciliation (watch examples/local-dev.yaml)
```

### Building
```bash
go build -ldflags="-X github.com/dataplanelabs/gcplane/cmd.Version=$(git describe --tags)" -o gcplane .
```

### CI/CD Trigger
- **CI**: Push to any branch → lint + test + build
- **E2E**: CI passes → reset GoClaw 1.2.0 + run integration tests
- **Release**: Tag `v*` → build + publish multi-platform (Linux, macOS, Windows) + Docker (ghcr.io)
- **Upstream Check**: Weekly cron → verify GoClaw compatibility

## Notable Implementation Details

### 1. Diff & Export Are Implemented
- `diff` command (v0.2.0+): Compare manifest vs live state, show field-level diffs
- `export` command (v0.2.0+): Dump GoClaw state as manifest YAML (with --all flag)

### 2. Agent Key Resolution (v0.7.2+)
CronJob `agentKey` field (human-readable name) now resolved in both create and update paths via `agent_id` UUID injection.

### 3. Tenant UUID Caching
Multi-tenant mode caches tenant slug → UUID mapping to avoid redundant API calls during bulk operations.

### 4. stripInternal Helper
Removes API-internal fields (`tenant_id`, `tenant_name`, `tenant_slug`) from observe results to prevent contamination of manifest comparisons.

### 5. Field Masking Skip
Comparator identifies `"***"` values in GoClaw responses (API-masked sensitive fields) and skips them in diff calculation.

### 6. Composite Expansion
Text/template-based abstraction: `CompositeDefinition` expanded at load time (e.g., ChatBot → Agent + Channel).

### 7. Label Filtering
Manifest resources tagged with labels (e.g., `app: web`); GCPlane reconciles via `--label app=web` flag matching.

## Dependencies Management

### Direct Dependencies (5)
| Package | Version | Usage |
|---------|---------|-------|
| gorilla/websocket | v1.5.3 | WebSocket RPC |
| cobra | v1.10.2 | CLI |
| gopkg.in/yaml.v3 | v3.0.1 | YAML parsing |
| tview | v0.42.0 | Terminal UI |
| tcell/v2 | v2.8.1 | Terminal rendering |

### Indirect Dependencies
- Go stdlib: net, http, encoding/json, os, io, context, sync, fmt, time, log/slog, etc.

### Dependency Management
- Go modules (go.mod, go.sum)
- No automatic updates; reviewed on release
- Upstream compatibility checked weekly (see CI/CD workflows)

## Performance Characteristics

### Reconciliation
- **Observe Phase**: Parallel HTTP + WS requests (concurrent within package)
- **Compare Phase**: Deep spec comparison (skipfield optimization for write-only)
- **Act Phase**: Dependency-ordered sequential execution (parallel per-level possible)
- **Typical Cycle**: 30s default interval (serve mode)

### Resource Listing
- **HTTP Resources**: GET /resources endpoint, single call
- **WebSocket Resources**: RPC call per kind (lazy WS init)
- **Tenant Filtering**: Applied post-fetch (cached UUID resolution)

### Memory
- No persistent state; manifest + live state in memory during reconciliation
- TUI refresh: incremental state update (atomic writes)
- WS connection pooling (one per provider instance)

## Security Considerations

### Credential Masking
- API keys returned as `"***"` from GoClaw; GCPlane respects masking
- Secrets never logged (log/slog structured logging with redaction)

### Tenant Isolation
- Multi-tenant HTTP headers (X-GoClaw-Tenant-Id) enforced on all requests
- Observation filtering prevents cross-tenant data leakage
- Tenant UUID resolution cached to prevent enumeration attacks

### Resource Ownership
- Only gcplane-managed resources (created_by=gcplane) pruned
- User confirmation required before destructive operations

### Secret Resolution
- ${ENV_VAR} and file:// patterns resolved securely
- SOPS support for encrypted secrets in git
- Resolved at reconciliation time, not load

## Known Limitations & Edge Cases

1. **Skill Not Deletable**: GoClaw manages this; GCPlane can only update
2. **Masked Fields**: Comparison skips API-masked fields (*** values); may miss real drift if GoClaw API changes masking behavior
3. **Natural Key Assumption**: GCPlane assumes resource `name` fields are unique within scope; duplicate names cause undefined behavior
4. **WS Lazy Init**: WebSocket connection established on first WS resource access; failures here propagate
5. **Composite Expansion**: One-shot at load time; no dynamic re-composition on serve mode changes
6. **SystemConfig, SkillConfig, BuiltinToolConfig, MCPCredentials**: Not enumerable for prune (GoClaw design limitation)

## Related Documentation
- [`./project-roadmap.md`](./project-roadmap.md) — Version history & feature timeline
- [`./system-architecture.md`](./system-architecture.md) — Design patterns & data flow
- [`./code-standards.md`](./code-standards.md) — Naming, patterns, testing conventions
- [`./usage-guide.md`](./usage-guide.md) — CLI commands & deployment
- [`./manifest-reference.md`](./manifest-reference.md) — Resource schema & examples
- [`./tenant-structure.md`](./tenant-structure.md) — Multi-tenant patterns & layouts
