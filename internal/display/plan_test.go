package display

import (
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
)

// TestPrintPlan_NoPanic verifies that PrintPlan does not panic for a mixed plan
// containing create and noop actions, both in verbose and non-verbose modes.
func TestPrintPlan_NoPanic(t *testing.T) {
	plan := &reconciler.Plan{
		Changes: []reconciler.Change{
			{Kind: manifest.KindProvider, Name: "test", Action: reconciler.ActionCreate},
			{Kind: manifest.KindAgent, Name: "bot", Action: reconciler.ActionNoop},
		},
		Creates: 1,
		Noops:   1,
	}

	// Both verbose modes must not panic
	PrintPlan(plan, true)
	PrintPlan(plan, false)
}

// TestPrintPlan_WithErrors_NoPanic verifies that PrintPlan renders plan-level errors
// without panicking.
func TestPrintPlan_WithErrors_NoPanic(t *testing.T) {
	plan := &reconciler.Plan{
		Changes: []reconciler.Change{
			{Kind: manifest.KindProvider, Name: "bad", Action: reconciler.ActionNoop, Error: "observe failed"},
		},
		Errors: []string{"reconcile aborted: connection refused"},
		Noops:  1,
	}
	PrintPlan(plan, true)
}

// TestPrintPlan_AllActions_NoPanic verifies that PrintPlan handles every action
// type (create, update, delete, noop) without panicking.
func TestPrintPlan_AllActions_NoPanic(t *testing.T) {
	plan := &reconciler.Plan{
		Changes: []reconciler.Change{
			{Kind: manifest.KindProvider, Name: "create-me", Action: reconciler.ActionCreate},
			{
				Kind:   manifest.KindAgent,
				Name:   "update-me",
				Action: reconciler.ActionUpdate,
				Diff: map[string]reconciler.FieldDiff{
					"displayName": {Old: "old", New: "new"},
				},
			},
			{Kind: manifest.KindChannel, Name: "delete-me", Action: reconciler.ActionDelete},
			{Kind: manifest.KindMCPServer, Name: "unchanged", Action: reconciler.ActionNoop},
		},
		Creates: 1,
		Updates: 1,
		Deletes: 1,
		Noops:   1,
	}
	PrintPlan(plan, true)
}

// TestPrintDiff_NoPanic verifies that PrintDiff does not panic for a plan where
// all resources are in sync.
func TestPrintDiff_NoPanic(t *testing.T) {
	plan := &reconciler.Plan{
		Changes: []reconciler.Change{
			{Kind: manifest.KindProvider, Name: "test", Action: reconciler.ActionNoop},
		},
		Noops: 1,
	}
	PrintDiff(plan)
}

// TestPrintDiff_WithDrift_NoPanic verifies that PrintDiff renders all drift
// action types without panicking.
func TestPrintDiff_WithDrift_NoPanic(t *testing.T) {
	plan := &reconciler.Plan{
		Changes: []reconciler.Change{
			{Kind: manifest.KindProvider, Name: "missing", Action: reconciler.ActionCreate},
			{
				Kind:   manifest.KindAgent,
				Name:   "drifted",
				Action: reconciler.ActionUpdate,
				Diff: map[string]reconciler.FieldDiff{
					"model": {Old: "gpt-3.5", New: "gpt-4"},
				},
			},
			{Kind: manifest.KindChannel, Name: "orphan", Action: reconciler.ActionDelete},
			{Kind: manifest.KindMCPServer, Name: "ok", Action: reconciler.ActionNoop},
			{Kind: manifest.KindTool, Name: "errored", Action: reconciler.ActionNoop, Error: "observe failed"},
		},
		Creates: 1,
		Updates: 1,
		Deletes: 1,
		Noops:   2,
	}
	PrintDiff(plan)
}

// TestPrintPruneWarning_NoPanic verifies that PrintPruneWarning does not panic.
func TestPrintPruneWarning_NoPanic(t *testing.T) {
	PrintPruneWarning(3)
}

// TestPrintApplyResult_NoPanic verifies that PrintApplyResult does not panic
// for both successful and partially-failed results.
func TestPrintApplyResult_NoPanic(t *testing.T) {
	PrintApplyResult(&reconciler.ApplyResult{Applied: 5, Failed: 0})
	PrintApplyResult(&reconciler.ApplyResult{
		Applied: 3,
		Failed:  2,
		Errors:  []string{"failed to create agent", "network timeout"},
	})
}
