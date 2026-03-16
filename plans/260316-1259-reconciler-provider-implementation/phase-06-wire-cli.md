---
phase: 6
priority: critical
effort: S
status: pending
---

# Phase 6: Wire CLI Commands

## Overview

Connect the plan/apply/validate commands to the reconciler engine.

## Related Code Files

- **Modify:** `cmd/plan.go` — wire to reconciler dry-run
- **Modify:** `cmd/apply.go` — wire to reconciler apply
- **Modify:** `cmd/validate.go` — wire to manifest validator
- **Modify:** `cmd/root.go` — connection config resolution (flags > env > manifest)

## Implementation Steps

1. **Connection config resolution** (in root.go or new `cmd/config.go`):
   ```
   endpoint: --endpoint flag > GCPLANE_ENDPOINT env > manifest.connection.endpoint
   token:    --token flag > GCPLANE_TOKEN env > manifest.connection.token
   ```
   Resolve secrets in connection values too.

2. **cmd/validate.go**:
   - Load manifest via `manifest.Load(configFile)`
   - Run `manifest.Validate(m)`
   - Print errors or "valid"
   - No GoClaw connection needed

3. **cmd/plan.go**:
   - Load + validate manifest
   - Resolve connection config
   - Create provider: `goclaw.New(endpoint, token)`
   - Create engine: `reconciler.NewEngine(provider)`
   - Run `engine.Reconcile(manifest, dryRun=true)`
   - Print plan using diff display (Phase 8)

4. **cmd/apply.go**:
   - Same as plan but `dryRun=false`
   - Print results (applied, failed, errors)
   - Add `--auto-approve` flag (skip confirmation prompt)
   - Default: show plan first, ask "Apply? [y/N]"

## Todo

- [ ] Connection config resolution (flags > env > manifest)
- [ ] Wire validate command
- [ ] Wire plan command (dry-run reconcile + display)
- [ ] Wire apply command (reconcile + confirm + execute)
- [ ] Compile check + manual test

## Success Criteria

- `gcplane validate -f examples/minimal.yaml` validates schema
- `gcplane plan -f examples/minimal.yaml --endpoint ... --token ...` shows diff
- `gcplane apply -f examples/minimal.yaml` applies changes with confirmation
