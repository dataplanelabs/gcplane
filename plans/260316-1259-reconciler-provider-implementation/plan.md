---
status: completed
created: 2026-03-16
branch: main
slug: reconciler-provider-implementation
---

# GCPlane Reconciler + GoClaw Provider Implementation

## Overview

Wire up the Observe→Compare→Act reconciliation engine and GoClaw HTTP/WS provider to make `gcplane plan` and `gcplane apply` functional end-to-end.

## Phases

| # | Phase | Priority | Effort | Status |
|---|-------|----------|--------|--------|
| 1 | [HTTP Client + Auth](phase-01-http-client.md) | Critical | S | done |
| 2 | [WS RPC Client](phase-02-ws-client.md) | Critical | M | done |
| 3 | [Provider: HTTP Resources](phase-03-provider-http.md) | Critical | M | done |
| 4 | [Provider: WS Resources](phase-04-provider-ws.md) | High | M | done |
| 5 | [Reconciler Engine](phase-05-reconciler-engine.md) | Critical | M | done |
| 6 | [Wire CLI Commands](phase-06-wire-cli.md) | Critical | S | done |
| 7 | [SQLite State Store](phase-07-state-sqlite.md) | Medium | S | done |
| 8 | [Diff Display](phase-08-diff-display.md) | Medium | S | done |

## Dependencies

```
Phase 1 (HTTP Client) ──┐
                         ├──→ Phase 3 (HTTP Resources) ──┐
Phase 2 (WS Client) ────┤                                 ├──→ Phase 5 (Reconciler) → Phase 6 (CLI) → Phase 8 (Diff)
                         └──→ Phase 4 (WS Resources)  ───┘
                                                                Phase 7 (SQLite) ──→ Phase 6
```

## Key Decisions

- **HTTP-first**: Agents, providers, channels, MCP, skills, tools via REST
- **WS fallback**: Cron, teams, TTS via WebSocket RPC (no HTTP endpoints in GoClaw)
- **Natural keys**: agent_key, provider name, channel name, cron id
- **No delete**: Only create/update — unmanaged resources ignored
- **Provider lookup quirk**: Providers/channels UUID-only GET; must list+filter by name
- **API key masking**: GoClaw returns "***" for secrets; on update, preserve old value
