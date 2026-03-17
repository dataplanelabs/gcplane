package tui

import (
	"testing"
	"time"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
	"github.com/dataplanelabs/gcplane/internal/tui/views"
)

// TestSmokeLocalDev simulates the full TUI data flow without needing a TTY.
func TestSmokeLocalDev(t *testing.T) {
	// Simulate a manifest with resources like local-dev.yaml
	m := &manifest.Manifest{
		Metadata: manifest.Metadata{Name: "local-dev", Environment: "dev"},
		Resources: []manifest.Resource{
			{Kind: manifest.KindProvider, Name: "zai-coding"},
			{Kind: manifest.KindProvider, Name: "openrouter"},
			{Kind: manifest.KindProvider, Name: "anthropic"},
			{Kind: manifest.KindProvider, Name: "gemini"},
			{Kind: manifest.KindAgent, Name: "assistant"},
			{Kind: manifest.KindAgent, Name: "data-engineer"},
			{Kind: manifest.KindAgent, Name: "data-analyst"},
			{Kind: manifest.KindAgent, Name: "report-builder"},
			{Kind: manifest.KindAgent, Name: "dev-lead"},
			{Kind: manifest.KindChannel, Name: "telegram-main"},
			{Kind: manifest.KindChannel, Name: "slack-main"},
			{Kind: manifest.KindMCPServer, Name: "huly-mcp"},
			{Kind: manifest.KindCronJob, Name: "daily-standup"},
			{Kind: manifest.KindAgentTeam, Name: "data-team"},
			{Kind: manifest.KindAgentTeam, Name: "product-team"},
		},
	}

	// Simulate a plan from dry-run reconciliation
	plan := &reconciler.Plan{
		Changes: []reconciler.Change{
			{Kind: manifest.KindProvider, Name: "zai-coding", Action: reconciler.ActionNoop},
			{Kind: manifest.KindProvider, Name: "openrouter", Action: reconciler.ActionNoop},
			{Kind: manifest.KindProvider, Name: "anthropic", Action: reconciler.ActionUpdate,
				Diff: map[string]reconciler.FieldDiff{"apiBase": {Old: "old-url", New: "new-url"}}},
			{Kind: manifest.KindProvider, Name: "gemini", Action: reconciler.ActionCreate},
			{Kind: manifest.KindAgent, Name: "assistant", Action: reconciler.ActionNoop},
			{Kind: manifest.KindAgent, Name: "data-engineer", Action: reconciler.ActionNoop},
			{Kind: manifest.KindAgent, Name: "data-analyst", Action: reconciler.ActionUpdate,
				Diff: map[string]reconciler.FieldDiff{"model": {Old: "old-model", New: "gemini-2.5-flash"}}},
			{Kind: manifest.KindAgent, Name: "report-builder", Action: reconciler.ActionNoop},
			{Kind: manifest.KindAgent, Name: "dev-lead", Action: reconciler.ActionNoop},
			{Kind: manifest.KindChannel, Name: "telegram-main", Action: reconciler.ActionNoop},
			{Kind: manifest.KindChannel, Name: "slack-main", Action: reconciler.ActionCreate},
			{Kind: manifest.KindMCPServer, Name: "huly-mcp", Action: reconciler.ActionNoop},
			{Kind: manifest.KindCronJob, Name: "daily-standup", Action: reconciler.ActionNoop},
			{Kind: manifest.KindAgentTeam, Name: "data-team", Action: reconciler.ActionNoop},
			{Kind: manifest.KindAgentTeam, Name: "product-team", Action: reconciler.ActionNoop},
		},
		Creates: 2, Updates: 2, Noops: 11,
	}

	// Test Model
	model := NewModel(m, "http://localhost:18790", 10*time.Second)
	model.UpdatePlan(plan)

	// All changes
	changes := model.GetChanges()
	if len(changes) != 15 {
		t.Fatalf("want 15 changes, got %d", len(changes))
	}

	// Filter by kind
	model.SetKind(manifest.KindProvider)
	providerChanges := model.GetChanges()
	if len(providerChanges) != 4 {
		t.Fatalf("want 4 provider changes, got %d", len(providerChanges))
	}

	// Filter by search
	model.SetKind("")
	model.SetFilter("data")
	dataChanges := model.GetChanges()
	if len(dataChanges) != 3 {
		t.Fatalf("want 3 changes matching 'data', got %d", len(dataChanges))
	}

	// Test table rendering
	model.SetFilter("")
	table := views.NewResourceTable()
	table.Refresh(model.GetChanges())

	if table.Table.GetRowCount() != 16 { // 1 header + 15 data
		t.Fatalf("want 16 rows, got %d", table.Table.GetRowCount())
	}

	// Verify status summary
	summary := views.StatusSummary(model.GetChanges())
	if summary != "11 InSync  2 Drifted  2 Missing" {
		t.Fatalf("unexpected summary: %s", summary)
	}

	// Test drift view rendering (no crash)
	driftView := views.NewDriftView()
	driftedChange := plan.Changes[2] // anthropic — drifted
	driftView.Show(driftedChange)

	inSyncChange := plan.Changes[0] // zai-coding — in sync
	driftView.Show(inSyncChange)

	missingChange := plan.Changes[3] // gemini — missing
	driftView.Show(missingChange)

	// Test attach mode status conversion
	t.Run("StatusToChanges", func(t *testing.T) {
		changes := StatusToChanges(nil)
		if changes != nil {
			t.Error("want nil for nil status")
		}
	})

	t.Log("Smoke test passed — all TUI components work correctly")
}
