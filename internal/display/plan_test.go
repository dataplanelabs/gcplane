package display

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
)

// captureOutput redirects display output to a buffer and returns the captured string.
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	old := output
	output = &buf
	defer func() { output = old }()
	fn()
	return buf.String()
}

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
			{Kind: manifest.KindSkill, Name: "errored", Action: reconciler.ActionNoop, Error: "observe failed"},
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

// --- Content validation tests ---

func TestPrintCreate_Output(t *testing.T) {
	t.Parallel()
	out := captureOutput(t, func() {
		printCreate(reconciler.Change{Kind: manifest.KindAgent, Name: "my-bot", Action: reconciler.ActionCreate})
	})
	if !strings.Contains(out, "+ Agent/my-bot") {
		t.Errorf("expected '+ Agent/my-bot', got %q", out)
	}
}

func TestPrintUpdate_Output(t *testing.T) {
	t.Parallel()
	out := captureOutput(t, func() {
		printUpdate(reconciler.Change{
			Kind: manifest.KindProvider, Name: "anthropic", Action: reconciler.ActionUpdate,
			Diff: map[string]reconciler.FieldDiff{
				"model": {Old: "gpt-3.5", New: "gpt-4"},
			},
		})
	})
	if !strings.Contains(out, "~ Provider/anthropic") {
		t.Errorf("expected '~ Provider/anthropic', got %q", out)
	}
	if !strings.Contains(out, "model:") {
		t.Errorf("expected diff key 'model:', got %q", out)
	}
}

func TestPrintUpdate_Forced(t *testing.T) {
	t.Parallel()
	out := captureOutput(t, func() {
		printUpdate(reconciler.Change{
			Kind: manifest.KindAgent, Name: "bot", Action: reconciler.ActionUpdate, Forced: true,
		})
	})
	if !strings.Contains(out, "(force)") {
		t.Errorf("expected '(force)' in forced update, got %q", out)
	}
}

func TestPrintDelete_Output(t *testing.T) {
	t.Parallel()
	out := captureOutput(t, func() {
		printDelete(reconciler.Change{Kind: manifest.KindChannel, Name: "slack", Action: reconciler.ActionDelete})
	})
	if !strings.Contains(out, "- Channel/slack") {
		t.Errorf("expected '- Channel/slack', got %q", out)
	}
}

func TestPrintNoop_InSync(t *testing.T) {
	t.Parallel()
	out := captureOutput(t, func() {
		printNoop(reconciler.Change{Kind: manifest.KindAgent, Name: "bot", Action: reconciler.ActionNoop})
	})
	if !strings.Contains(out, "= Agent/bot") {
		t.Errorf("expected '= Agent/bot', got %q", out)
	}
	if !strings.Contains(out, "no changes") {
		t.Errorf("expected 'no changes', got %q", out)
	}
}

func TestPrintNoop_WithError(t *testing.T) {
	t.Parallel()
	out := captureOutput(t, func() {
		printNoop(reconciler.Change{Kind: manifest.KindAgent, Name: "bot", Action: reconciler.ActionNoop, Error: "timeout"})
	})
	if !strings.Contains(out, "! Agent/bot") {
		t.Errorf("expected '! Agent/bot', got %q", out)
	}
	if !strings.Contains(out, "skipped: timeout") {
		t.Errorf("expected 'skipped: timeout', got %q", out)
	}
}

func TestPrintPlan_SummaryLine(t *testing.T) {
	t.Parallel()
	plan := &reconciler.Plan{
		Changes: []reconciler.Change{
			{Kind: manifest.KindProvider, Name: "a", Action: reconciler.ActionCreate},
			{Kind: manifest.KindAgent, Name: "b", Action: reconciler.ActionUpdate, Diff: map[string]reconciler.FieldDiff{"x": {}}},
			{Kind: manifest.KindChannel, Name: "c", Action: reconciler.ActionDelete},
		},
		Creates: 1, Updates: 1, Deletes: 1,
	}
	out := captureOutput(t, func() { PrintPlan(plan, false) })
	if !strings.Contains(out, "1 to create") || !strings.Contains(out, "1 to update") || !strings.Contains(out, "1 to delete") {
		t.Errorf("summary line missing counts, got %q", out)
	}
}

func TestPrintDiff_NoChange(t *testing.T) {
	t.Parallel()
	plan := &reconciler.Plan{
		Changes: []reconciler.Change{
			{Kind: manifest.KindProvider, Name: "ok", Action: reconciler.ActionNoop},
		},
		Noops: 1,
	}
	out := captureOutput(t, func() { PrintDiff(plan) })
	if !strings.Contains(out, "No drift detected") {
		t.Errorf("expected 'No drift detected', got %q", out)
	}
}

func TestFormatVal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input any
		want  string
	}{
		{nil, "(none)"},
		{"hello", "hello"},
		{strings.Repeat("x", 100), strings.Repeat("x", 77) + "..."},
	}
	for _, c := range cases {
		got := formatVal(c.input)
		if got != c.want {
			t.Errorf("formatVal(%v) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestSortedKeys(t *testing.T) {
	t.Parallel()
	m := map[string]reconciler.FieldDiff{
		"z": {}, "a": {}, "m": {},
	}
	keys := sortedKeys(m)
	if len(keys) != 3 || keys[0] != "a" || keys[1] != "m" || keys[2] != "z" {
		t.Errorf("expected [a m z], got %v", keys)
	}
}
