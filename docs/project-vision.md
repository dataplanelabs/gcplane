# GCPlane — Project Vision

> Declarative GitOps control plane for AI agent platforms.
> One binary. One manifest. Any platform.

## Mission

Make AI agent infrastructure as manageable as Kubernetes workloads. Declare desired state in YAML, let gcplane reconcile reality.

## Core Pillars

### 1. Declarative GitOps
Everything is a YAML manifest. State lives in git. Reconciliation is automatic. No imperative scripts, no click-ops. The manifest is the single source of truth.

### 2. Deploy-Anywhere
Single statically-linked binary. Runs on laptop, VPS (systemd), Docker, Kubernetes. No platform lock-in, no cloud dependencies. Config via env vars + YAML.

### 3. Minimal Dependencies
Under 10 direct Go dependencies. Stdlib preferred. No SDKs for simple HTTP calls. Every dependency must justify its existence.

### 4. Provider Abstraction (Terraform Model)
`provider/goclaw` is the first provider. The `Provider` interface is the abstraction boundary. Future providers (LangGraph, CrewAI, Dify, custom) implement the same interface. Manifest format stays stable — providers translate.

### 5. Observable by Default
Every reconciliation emits structured logs (slog). Metrics exported via Prometheus. TUI provides real-time introspection. Users should never wonder "what did gcplane just do?"

## Progressive User Model

| Persona | Use Case | Features |
|---------|----------|----------|
| **Solo dev** | 1 GoClaw instance, local dev | `apply`, `plan`, `top`, single manifest |
| **Small team** | Shared git repo, CI/CD | `serve`, git source, webhooks, multi-env |
| **Platform team** | Multi-tenant, audit, compliance | `--tenants-dir`, per-tenant status, drift alerts |

Core stays lean for solo. Features layer on for teams. Never sacrifice simplicity for enterprise.

## TUI Vision — k9s for AI Agents

The TUI (`gcplane top`) is a **read-heavy, write-light** resource browser. North star: k9s.

### What it IS
- Real-time resource browser with status coloring
- Drill-down views: YAML detail, drift diff, reconciliation trace
- Quick actions: apply, delete, edit (with confirmation)
- Vim-style navigation, command mode, search/filter
- Multi-tenant awareness (`:tenant` switching)

### What it is NOT
- Not a full workflow tool (use `apply`/`plan` CLI for that)
- Not a dashboard with charts/graphs (emit metrics, don't visualize)
- Not an IDE or manifest editor

### TUI Architecture Target
Extensible view system with:
- **View registry**: views register themselves, discoverable by keybinding
- **Event bus**: decouple data producers (reconciler, provider) from consumers (views)
- **Trace view**: reconciliation log, API calls, drift history, error context
- **Plugin-friendly**: new views can be added without modifying core wiring

## Stability Contract (v1.x)

| Layer | Stability | Policy |
|-------|-----------|--------|
| **YAML manifest schema** | Frozen | Additive only. No breaking field changes. |
| **CLI commands + flags** | Stable | No flag removal or rename. New flags OK. |
| **Internal Go packages** | Unstable | Free to refactor. Not a public API. |
| **Provider interface** | Evolving | May change with new resource kinds. Not published as SDK. |

## Out of Scope (Anti-Patterns)

These are explicit NO decisions. Violating them requires a major version bump and team consensus.

### NOT a GUI / Web Dashboard
No browser UI, no React, no HTTP frontend for humans. GoClaw has its own web UI. gcplane is terminal-first. The TUI is the interactive interface.

### NOT a GoClaw SDK
gcplane is a **tool**, not a **library**. Don't expose `internal/` packages for programmatic access. If someone needs a Go client for GoClaw, that's a separate project.

### NOT a Monitoring System
Emit Prometheus metrics and Slack alerts — don't build dashboards. Don't store time-series data. Don't replace Grafana. The TUI shows **current state**, not historical trends.

### NOT a Configuration Management Tool
Don't manage infrastructure (servers, networks, DNS). Only manage AI agent platform resources. Ansible/Terraform handle infra; gcplane handles the app layer above.

### Scope Boundary Heuristic
> If the feature requires storing persistent state beyond what the AI platform API provides, it's probably out of scope.

## Architecture Invariants

These hold across all versions and providers:

1. **No local state** — The AI platform API is the source of truth. gcplane carries no database.
2. **Idempotent reconciliation** — Running `apply` twice produces the same result.
3. **Dependency-ordered processing** — Resources created in dependency order, deleted in reverse.
4. **Secret resolution at runtime** — `${ENV}` and `file://` resolved at reconciliation time, never persisted.
5. **Natural key addressing** — Humans use names, gcplane resolves to platform-internal IDs.
6. **Continue on error** — One resource failure doesn't block others in the same reconciliation.

## Roadmap Milestones

### v1.2 — TUI Extensibility + Trace
- Refactor TUI to view registry pattern
- Add trace/log view (reconciliation steps, API calls, errors)
- Add drift history view (timeline of corrections)
- Extract vim-scroll handler (DRY)

### v1.3 — Provider Interface Formalization
- Extract `Provider` interface to its own package
- Document provider contract (Observe, Create, Update, Delete, List, Close)
- Provider discovery/registration pattern
- Provider-specific resource kind mapping

### v1.4 — Advanced Observability
- Structured event emission (slog + event types)
- TUI API request/response inspector
- Export reconciliation events as JSON (for external consumption)
- OpenTelemetry trace context propagation (optional)

### v1.5 — Multi-Provider Foundation
- Second provider skeleton (e.g., Dify or LangGraph)
- Manifest `provider` field routing
- Cross-provider resource references
- Provider capability negotiation (which resource kinds supported)

### Future (v2.x considerations)
- Plugin system for custom resource kinds
- Policy engine (OPA/Rego for manifest validation)
- Collaborative features (locking, approval workflows)
- Offline mode (cached state for disconnected environments)
