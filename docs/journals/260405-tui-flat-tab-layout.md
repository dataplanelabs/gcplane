# TUI Flat Tab Layout Refactor Complete

**Date**: 2026-04-05 14:30
**Severity**: Medium
**Component**: internal/tui (layout, navigation, keybindings)
**Status**: Resolved

## What Happened

Replaced 13-row toggleable bottom panel with 4 full-screen peer tabs (State/Logs/Events/Trace). Four-phase sequential refactor over 2 days: ViewRegistry extension → panel registration → keybinding rewire → cleanup. 385 tests pass, zero race conditions.

## The Brutal Truth

This was necessary but exhausting. The bottom panel toggling was cramped and the mental model felt wrong — three separate systems (PanelTab, overlay stack, command bar) fighting for the same 13 rows. Flattening to tabs felt obvious in hindsight but required careful scaffolding to avoid breaking compilation mid-refactor.

The code review caught three subtle bugs we would have shipped: a race condition reading ActiveTab(), an incomplete event filter, and a nil-guard miss on trace view. That stung.

## Technical Details

**Phase 1**: Extended ViewRegistry with `PrimaryTab` model. Added `SwitchTab()`, `HasOverlay()`, renamed internal `viewStack` to `overlayStack`. Temporary `PanelTab*` prefix for constants during coexistence.

**Phase 2**: Registered LogsPanel and EventsPanel as `View` implementors. Static 3-row layout (2-row header + pages + cmdbar). Added `switchTab()` with log tail lifecycle, `tabRefreshLoop` for periodic updates.

**Phase 3**: Rewired keybindings — S/L/E/T for tab switching (guarded by `!HasOverlay()`), context-sensitive number keys per tab, remapped e→Ctrl+E for edit, removed deprecated Ctrl+L/Alt+1/2/3.

**Phase 4**: Deleted `bottom_panel.go` (121 lines), removed all compat shims and dead code paths.

**Code review fixes**:
- Event filter needed `strings.HasPrefix()` for "team." prefix matching, not exact equality
- `ActiveTab()` read moved inside `QueueUpdateDraw()` for race safety
- Added nil guard for `switchTab(TabTrace)` when traceView uninitialized
- Logs/Events pause renders `[PAUSED]` instead of early return

## What We Tried

Initially attempted in-place mutation of bottom panel. Abandoned after two hours — too much state coupling. Switched to scaffolding approach: new tabs coexist, old panel routes gradually migrate, then full removal. This took longer upfront but reduced risk of half-broken intermediate states.

## Root Cause Analysis

The bottom panel was a workaround — it tried to cram three independent views into scarce real estate without proper abstraction. The real issue was that ViewRegistry's overlay model didn't extend to "primary content areas" — only modals. Building PrimaryTab as a first-class citizen in the registry fixed the architecture.

## Lessons Learned

- Phased refactors need temporary naming (`PanelTab*` prefix) to maintain compilability. This buys confidence.
- Code review on refactors catches race conditions and nil guards that fresh eyes spot immediately. Never skip this.
- `QueueUpdateDraw()` boundaries matter for thread safety — reads of shared state belong inside, not before.
- Extracting shared helpers (`focusActiveTab()`) early prevents keybinding duplication and inconsistency.

## Next Steps

- Monitor trace view initialization in edge cases (nil guard is defensive but should be validated in e2e tests)
- Docs are current (CLAUDE.md, usage-guide.md, system-architecture.md updated)
- Commit 086c476 is stable; ready for release in v1.2.1 if needed

**Metrics**: 18 files changed, 1526 insertions, 221 deletions, 385 tests passing, 0 race conditions detected.
