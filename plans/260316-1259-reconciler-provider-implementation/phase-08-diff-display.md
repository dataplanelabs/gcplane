---
phase: 8
priority: medium
effort: S
status: pending
---

# Phase 8: Diff Display

## Overview

Terraform-style colored output for plan results.

## Related Code Files

- **Create:** `internal/display/plan.go` — plan output formatting

## Implementation Steps

1. Create `display/plan.go` with `PrintPlan(plan *reconciler.Plan)`:
   ```
   GCPlane Plan: 2 to create, 1 to update, 3 unchanged

   + Provider/anthropic
       displayName: "Anthropic"
       baseUrl: https://api.anthropic.com

   ~ Agent/support-bot
       model: "claude-sonnet-4-20250514" → "claude-haiku-4-5-20251001"

   = ChannelInstance/telegram-main (no changes)
   ```

2. Color codes (ANSI):
   - `+` green — create
   - `~` yellow — update
   - `=` dim/gray — noop (only in verbose mode)
   - Field diffs: red for old, green for new

3. Summary line at top and bottom

4. `PrintApplyResult(result *reconciler.ApplyResult)`:
   - "Apply complete! 2 created, 1 updated, 0 failed."
   - List errors if any

5. Respect `--verbose` flag for showing noop resources

## Todo

- [ ] Create display/plan.go
- [ ] Implement PrintPlan with colored diff output
- [ ] Implement PrintApplyResult
- [ ] Wire into plan/apply commands
- [ ] Compile check

## Success Criteria

- Clean, readable terraform-style output
- Color-coded changes
- Summary counts
- Verbose mode shows unchanged resources
