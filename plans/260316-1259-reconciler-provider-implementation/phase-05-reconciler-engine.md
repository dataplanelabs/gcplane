---
phase: 5
priority: critical
effort: M
status: pending
---

# Phase 5: Reconciler Engine

## Overview

Core Observe→Compare→Act engine. Takes a manifest + provider, produces a Plan (dry-run) or executes changes (apply).

## Related Code Files

- **Create:** `internal/reconciler/engine.go` — reconciliation engine
- **Create:** `internal/reconciler/compare.go` — deep comparison logic
- **Modify:** `internal/reconciler/types.go` — add Engine interface if needed

## Key Insights

- Resources must be processed in dependency order (providers → agents → channels/cron)
- Compare: deep-diff desired spec vs observed state, ignoring masked fields ("***")
- Secrets must be resolved before comparison
- Plan = list of Changes; Apply = execute Changes via provider

## Implementation Steps

1. Create `Engine` struct:
   ```go
   type Engine struct {
       provider Provider
       secrets  *secrets.Resolver  // or use secrets.Resolve directly
   }

   type Provider interface {
       Observe(kind manifest.ResourceKind, key string) (map[string]any, error)
       Create(kind manifest.ResourceKind, key string, spec map[string]any) error
       Update(kind manifest.ResourceKind, key string, spec map[string]any) error
   }
   ```

2. Implement `Reconcile(m *manifest.Manifest, dryRun bool) (*Plan, error)`:
   - Group resources by kind
   - Process in `manifest.ApplyOrder()` sequence
   - For each resource:
     a. Resolve secrets in spec (`${ENV}`, `file://`)
     b. Call `provider.Observe(kind, key)` → get current state
     c. If nil → action=create
     d. If exists → compare spec vs observed → if diff → action=update
     e. If no diff → action=noop
     f. Record Change
   - If `dryRun=false`, execute creates/updates via provider
   - Return Plan with summary counts

3. Create `compare.go`:
   - `CompareSpec(desired, actual map[string]any) map[string]FieldDiff`
   - Deep recursive comparison
   - Skip fields where actual="***" (masked secrets)
   - Handle nested maps and slices
   - Return only changed fields

4. Secret resolution on spec:
   - Walk spec map recursively
   - For string values, call `secrets.Resolve(val)`
   - Replace in-place (work on a copy of spec)

## Todo

- [ ] Create engine.go with Engine struct
- [ ] Implement Reconcile method (dry-run + apply mode)
- [ ] Create compare.go with deep spec comparison
- [ ] Secret resolution walker for spec maps
- [ ] Handle dependency ordering via ApplyOrder()
- [ ] Compile check

## Success Criteria

- Engine produces correct Plan for create/update/noop scenarios
- Masked fields ("***") skipped in comparison
- Resources processed in dependency order
- Secrets resolved before comparison
- Apply mode executes changes via provider
