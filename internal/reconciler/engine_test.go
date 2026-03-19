package reconciler

import (
	"fmt"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

// mockProvider implements ProviderInterface for testing.
type mockProvider struct {
	observed map[string]map[string]any
	created  []string
	updated  []string
}

func newMockProvider() *mockProvider {
	return &mockProvider{
		observed: make(map[string]map[string]any),
	}
}

func (m *mockProvider) Observe(kind manifest.ResourceKind, key string) (map[string]any, error) {
	uid := fmt.Sprintf("%s/%s", kind, key)
	state, ok := m.observed[uid]
	if !ok {
		return nil, nil
	}
	return state, nil
}

func (m *mockProvider) Create(kind manifest.ResourceKind, key string, spec map[string]any) error {
	uid := fmt.Sprintf("%s/%s", kind, key)
	m.created = append(m.created, uid)
	return nil
}

func (m *mockProvider) Update(kind manifest.ResourceKind, key string, spec map[string]any) error {
	uid := fmt.Sprintf("%s/%s", kind, key)
	m.updated = append(m.updated, uid)
	return nil
}

func (m *mockProvider) Delete(kind manifest.ResourceKind, key string) error {
	return nil
}

func (m *mockProvider) ListAll(kind manifest.ResourceKind) ([]ResourceInfo, error) {
	return nil, nil
}

func TestReconcile_CreateNew(t *testing.T) {
	provider := newMockProvider()
	engine := NewEngine(provider)

	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{Kind: manifest.KindProvider, Name: "anthropic", Spec: map[string]any{"displayName": "Anthropic"}},
		},
	}

	plan, _ := engine.Reconcile(m, ReconcileOpts{DryRun: true})
	if plan.Creates != 1 {
		t.Errorf("expected 1 create, got %d", plan.Creates)
	}
	if plan.Updates != 0 {
		t.Errorf("expected 0 updates, got %d", plan.Updates)
	}
}

func TestReconcile_UpdateExisting(t *testing.T) {
	provider := newMockProvider()
	provider.observed["Agent/bot"] = map[string]any{"model": "old-model"}

	engine := NewEngine(provider)
	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{Kind: manifest.KindAgent, Name: "bot", Spec: map[string]any{"model": "new-model"}},
		},
	}

	plan, _ := engine.Reconcile(m, ReconcileOpts{DryRun: true})
	if plan.Updates != 1 {
		t.Errorf("expected 1 update, got %d", plan.Updates)
	}
}

func TestReconcile_NoopIdentical(t *testing.T) {
	provider := newMockProvider()
	provider.observed["Provider/anthropic"] = map[string]any{"displayName": "Anthropic"}

	engine := NewEngine(provider)
	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{Kind: manifest.KindProvider, Name: "anthropic", Spec: map[string]any{"displayName": "Anthropic"}},
		},
	}

	plan, _ := engine.Reconcile(m, ReconcileOpts{DryRun: true})
	if plan.Noops != 1 {
		t.Errorf("expected 1 noop, got %d", plan.Noops)
	}
}

func TestReconcile_ApplyExecutes(t *testing.T) {
	provider := newMockProvider()
	engine := NewEngine(provider)

	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{Kind: manifest.KindProvider, Name: "openai", Spec: map[string]any{"name": "OpenAI"}},
		},
	}

	_, result := engine.Reconcile(m, ReconcileOpts{DryRun: false})
	if result.Applied != 1 {
		t.Errorf("expected 1 applied, got %d", result.Applied)
	}
	if len(provider.created) != 1 {
		t.Errorf("expected 1 create call, got %d", len(provider.created))
	}
}

func TestReconcile_ForceUpdatesIdentical(t *testing.T) {
	provider := newMockProvider()
	provider.observed["Provider/anthropic"] = map[string]any{"displayName": "Anthropic"}

	engine := NewEngine(provider)
	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{Kind: manifest.KindProvider, Name: "anthropic", Spec: map[string]any{"displayName": "Anthropic"}},
		},
	}

	// Without force: noop
	plan, _ := engine.Reconcile(m, ReconcileOpts{DryRun: true})
	if plan.Noops != 1 {
		t.Errorf("expected 1 noop without force, got %d", plan.Noops)
	}

	// With force: update
	plan, _ = engine.Reconcile(m, ReconcileOpts{DryRun: true, Force: true})
	if plan.Updates != 1 {
		t.Errorf("expected 1 update with force, got %d", plan.Updates)
	}
	if plan.Noops != 0 {
		t.Errorf("expected 0 noops with force, got %d", plan.Noops)
	}
}

func TestReconcile_ForceApplyExecutes(t *testing.T) {
	provider := newMockProvider()
	provider.observed["Provider/anthropic"] = map[string]any{"displayName": "Anthropic"}

	engine := NewEngine(provider)
	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{Kind: manifest.KindProvider, Name: "anthropic", Spec: map[string]any{"displayName": "Anthropic"}},
		},
	}

	// Force apply should call Update even when specs are identical
	_, result := engine.Reconcile(m, ReconcileOpts{DryRun: false, Force: true})
	if result.Applied != 1 {
		t.Errorf("expected 1 applied with force, got %d", result.Applied)
	}
	if len(provider.updated) != 1 {
		t.Errorf("expected 1 update call with force, got %d", len(provider.updated))
	}
}

func TestReconcile_DependencyOrder(t *testing.T) {
	provider := newMockProvider()
	engine := NewEngine(provider)

	// Agent depends on Provider — Provider should be processed first
	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{Kind: manifest.KindAgent, Name: "bot", Spec: map[string]any{"provider": "anthropic"}},
			{Kind: manifest.KindProvider, Name: "anthropic", Spec: map[string]any{"name": "Anthropic"}},
		},
	}

	plan, _ := engine.Reconcile(m, ReconcileOpts{DryRun: true})
	if len(plan.Changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(plan.Changes))
	}
	// Provider should come first in changes due to ApplyOrder
	if plan.Changes[0].Kind != manifest.KindProvider {
		t.Errorf("expected Provider first, got %s", plan.Changes[0].Kind)
	}
	if plan.Changes[1].Kind != manifest.KindAgent {
		t.Errorf("expected Agent second, got %s", plan.Changes[1].Kind)
	}
}
