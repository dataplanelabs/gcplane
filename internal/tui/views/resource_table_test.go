package views

import (
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
)

func testChanges() []reconciler.Change {
	return []reconciler.Change{
		{Kind: manifest.KindProvider, Name: "openai", Action: reconciler.ActionNoop},
		{Kind: manifest.KindAgent, Name: "assistant", Action: reconciler.ActionUpdate,
			Diff: map[string]reconciler.FieldDiff{"model": {Old: "gpt-3", New: "gpt-4"}}},
		{Kind: manifest.KindAgent, Name: "helper", Action: reconciler.ActionCreate},
		{Kind: manifest.KindChannel, Name: "slack", Action: reconciler.ActionNoop},
	}
}

func TestActionToStatus(t *testing.T) {
	tests := []struct {
		change reconciler.Change
		want   string
	}{
		{reconciler.Change{Action: reconciler.ActionNoop}, "InSync"},
		{reconciler.Change{Action: reconciler.ActionUpdate}, "Drifted"},
		{reconciler.Change{Action: reconciler.ActionCreate}, "Missing"},
		{reconciler.Change{Action: reconciler.ActionDelete}, "Extra"},
		{reconciler.Change{Action: reconciler.ActionNoop, Error: "connection refused"}, "Error"},
	}

	for _, tt := range tests {
		got := actionToStatus(tt.change)
		if got != tt.want {
			t.Errorf("actionToStatus(%v) = %s, want %s", tt.change.Action, got, tt.want)
		}
	}
}

func TestToTableRow(t *testing.T) {
	c := reconciler.Change{
		Kind:   manifest.KindAgent,
		Name:   "assistant",
		Action: reconciler.ActionUpdate,
		Diff: map[string]reconciler.FieldDiff{
			"model":       {Old: "gpt-3", New: "gpt-4"},
			"temperature": {Old: 0.5, New: 0.7},
		},
	}

	row := toTableRow(c)
	if row.kind != manifest.KindAgent {
		t.Errorf("want Agent, got %s", row.kind)
	}
	if row.status != "Drifted" {
		t.Errorf("want Drifted, got %s", row.status)
	}
	// Drift info should list fields alphabetically
	if row.driftInfo != "model, temperature" {
		t.Errorf("want 'model, temperature', got '%s'", row.driftInfo)
	}
}

func TestToTableRowNoDrift(t *testing.T) {
	c := reconciler.Change{
		Kind:   manifest.KindProvider,
		Name:   "openai",
		Action: reconciler.ActionNoop,
	}
	row := toTableRow(c)
	if row.driftInfo != "-" {
		t.Errorf("want '-' for no drift, got '%s'", row.driftInfo)
	}
}

func TestStatusSummary(t *testing.T) {
	changes := testChanges()
	summary := StatusSummary(changes)
	// 2 InSync, 1 Drifted, 1 Missing
	if summary != "2 InSync  1 Drifted  1 Missing" {
		t.Errorf("unexpected summary: %s", summary)
	}
}

func TestStatusSummaryEmpty(t *testing.T) {
	summary := StatusSummary(nil)
	if summary != "no resources" {
		t.Errorf("want 'no resources', got '%s'", summary)
	}
}

func TestResourceTableRefresh(t *testing.T) {
	rt := NewResourceTable()
	changes := testChanges()

	rt.Refresh(changes)

	// Header + 4 data rows
	if rt.Table.GetRowCount() != 5 {
		t.Errorf("want 5 rows (1 header + 4 data), got %d", rt.Table.GetRowCount())
	}

	// Verify header
	headerCell := rt.Table.GetCell(0, 0)
	if headerCell.Text != "KIND" {
		t.Errorf("want KIND header, got %s", headerCell.Text)
	}
}

func TestResourceTableGetSelectedChange(t *testing.T) {
	rt := NewResourceTable()
	rt.Refresh(testChanges())

	// Select first data row
	rt.Table.Select(1, 0)
	c := rt.GetSelectedChange()
	if c == nil {
		t.Fatal("expected selected change, got nil")
	}
	// First row should be Provider (sorted by ApplyOrder)
	if c.Kind != manifest.KindProvider {
		t.Errorf("want Provider, got %s", c.Kind)
	}
}

func TestKindOrderMap(t *testing.T) {
	m := kindOrderMap()
	if m[manifest.KindProvider] >= m[manifest.KindAgent] {
		t.Error("Provider should come before Agent in order")
	}
}
