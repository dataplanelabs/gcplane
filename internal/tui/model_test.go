package tui

import (
	"testing"
	"time"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
)

func testManifest() *manifest.Manifest {
	return &manifest.Manifest{
		Metadata: manifest.Metadata{Name: "test-manifest"},
		Resources: []manifest.Resource{
			{Kind: manifest.KindProvider, Name: "openai"},
			{Kind: manifest.KindAgent, Name: "assistant"},
		},
	}
}

func testPlan() *reconciler.Plan {
	return &reconciler.Plan{
		Changes: []reconciler.Change{
			{Kind: manifest.KindProvider, Name: "openai", Action: reconciler.ActionNoop},
			{Kind: manifest.KindAgent, Name: "assistant", Action: reconciler.ActionUpdate,
				Diff: map[string]reconciler.FieldDiff{"model": {Old: "gpt-3", New: "gpt-4"}}},
			{Kind: manifest.KindAgent, Name: "helper", Action: reconciler.ActionCreate},
			{Kind: manifest.KindChannel, Name: "slack", Action: reconciler.ActionNoop},
		},
		Creates: 1, Updates: 1, Noops: 2,
	}
}

func TestNewModel(t *testing.T) {
	m := NewModel(testManifest(), "http://localhost:8080", 10*time.Second)
	if m.GetManifestName() != "test-manifest" {
		t.Errorf("want manifest name test-manifest, got %s", m.GetManifestName())
	}
	if m.GetEndpoint() != "http://localhost:8080" {
		t.Errorf("want endpoint http://localhost:8080, got %s", m.GetEndpoint())
	}
	if m.GetKind() != "" {
		t.Errorf("want empty kind, got %s", m.GetKind())
	}
	if m.GetInterval() != 10*time.Second {
		t.Errorf("want 10s interval, got %s", m.GetInterval())
	}
}

func TestModelUpdatePlan(t *testing.T) {
	m := NewModel(testManifest(), "http://localhost:8080", 10*time.Second)
	plan := testPlan()
	m.UpdatePlan(plan)

	if m.GetPlan() == nil {
		t.Fatal("plan should not be nil after update")
	}
	if m.GetError() != nil {
		t.Errorf("error should be nil after successful update")
	}
	if m.GetLastRefresh().IsZero() {
		t.Errorf("lastRefresh should be set after update")
	}
}

func TestModelGetChangesNoFilter(t *testing.T) {
	m := NewModel(testManifest(), "http://localhost:8080", 10*time.Second)
	m.UpdatePlan(testPlan())

	changes := m.GetChanges()
	if len(changes) != 4 {
		t.Errorf("want 4 changes, got %d", len(changes))
	}
}

func TestModelGetChangesKindFilter(t *testing.T) {
	m := NewModel(testManifest(), "http://localhost:8080", 10*time.Second)
	m.UpdatePlan(testPlan())

	m.SetKind(manifest.KindAgent)
	changes := m.GetChanges()
	if len(changes) != 2 {
		t.Errorf("want 2 agent changes, got %d", len(changes))
	}
	for _, c := range changes {
		if c.Kind != manifest.KindAgent {
			t.Errorf("want Agent kind, got %s", c.Kind)
		}
	}
}

func TestModelGetChangesSearchFilter(t *testing.T) {
	m := NewModel(testManifest(), "http://localhost:8080", 10*time.Second)
	m.UpdatePlan(testPlan())

	m.SetFilter("assist")
	changes := m.GetChanges()
	if len(changes) != 1 {
		t.Errorf("want 1 change matching 'assist', got %d", len(changes))
	}
	if len(changes) > 0 && changes[0].Name != "assistant" {
		t.Errorf("want assistant, got %s", changes[0].Name)
	}
}

func TestModelGetChangesCombinedFilter(t *testing.T) {
	m := NewModel(testManifest(), "http://localhost:8080", 10*time.Second)
	m.UpdatePlan(testPlan())

	m.SetKind(manifest.KindAgent)
	m.SetFilter("helper")
	changes := m.GetChanges()
	if len(changes) != 1 {
		t.Errorf("want 1 change (Agent+helper), got %d", len(changes))
	}
}

func TestModelGetChangesNilPlan(t *testing.T) {
	m := NewModel(testManifest(), "http://localhost:8080", 10*time.Second)
	changes := m.GetChanges()
	if changes != nil {
		t.Errorf("want nil changes for nil plan, got %v", changes)
	}
}
