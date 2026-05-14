package reconciler

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

// mockProvider implements ProviderInterface for testing.
type mockProvider struct {
	observed map[string]map[string]any
	created  []string
	updated  []string
	lastSpec map[string]any // captures last spec sent to Create/Update
}

func newMockProvider() *mockProvider {
	return &mockProvider{
		observed: make(map[string]map[string]any),
	}
}

func (m *mockProvider) Observe(_ context.Context, kind manifest.ResourceKind, key string) (map[string]any, error) {
	uid := fmt.Sprintf("%s/%s", kind, key)
	state, ok := m.observed[uid]
	if !ok {
		return nil, nil
	}
	return state, nil
}

func (m *mockProvider) Create(_ context.Context, kind manifest.ResourceKind, key string, spec map[string]any) error {
	uid := fmt.Sprintf("%s/%s", kind, key)
	m.created = append(m.created, uid)
	m.lastSpec = spec
	return nil
}

func (m *mockProvider) Update(_ context.Context, kind manifest.ResourceKind, key string, spec map[string]any) error {
	uid := fmt.Sprintf("%s/%s", kind, key)
	m.updated = append(m.updated, uid)
	m.lastSpec = spec
	return nil
}

func (m *mockProvider) Delete(_ context.Context, kind manifest.ResourceKind, key string) error {
	return nil
}

func (m *mockProvider) ListAll(_ context.Context, kind manifest.ResourceKind) ([]ResourceInfo, error) {
	return nil, nil
}

func TestReconcile_CreateNew(t *testing.T) {
	provider := newMockProvider()
	engine := NewEngine(provider, nil)

	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{Kind: manifest.KindProvider, Name: "anthropic", Spec: map[string]any{"displayName": "Anthropic"}},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
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

	engine := NewEngine(provider, nil)
	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{Kind: manifest.KindAgent, Name: "bot", Spec: map[string]any{"model": "new-model"}},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
	if plan.Updates != 1 {
		t.Errorf("expected 1 update, got %d", plan.Updates)
	}
}

func TestReconcile_NoopIdentical(t *testing.T) {
	provider := newMockProvider()
	provider.observed["Provider/anthropic"] = map[string]any{"displayName": "Anthropic"}

	engine := NewEngine(provider, nil)
	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{Kind: manifest.KindProvider, Name: "anthropic", Spec: map[string]any{"displayName": "Anthropic"}},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
	if plan.Noops != 1 {
		t.Errorf("expected 1 noop, got %d", plan.Noops)
	}
}

func TestReconcile_ApplyExecutes(t *testing.T) {
	provider := newMockProvider()
	engine := NewEngine(provider, nil)

	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{Kind: manifest.KindProvider, Name: "openai", Spec: map[string]any{"name": "OpenAI"}},
		},
	}

	_, result := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: false})
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

	engine := NewEngine(provider, nil)
	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{Kind: manifest.KindProvider, Name: "anthropic", Spec: map[string]any{"displayName": "Anthropic"}},
		},
	}

	// Without force: noop
	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
	if plan.Noops != 1 {
		t.Errorf("expected 1 noop without force, got %d", plan.Noops)
	}

	// With force: update
	plan, _ = engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true, Force: true})
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

	engine := NewEngine(provider, nil)
	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{Kind: manifest.KindProvider, Name: "anthropic", Spec: map[string]any{"displayName": "Anthropic"}},
		},
	}

	// Force apply should call Update even when specs are identical
	_, result := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: false, Force: true})
	if result.Applied != 1 {
		t.Errorf("expected 1 applied with force, got %d", result.Applied)
	}
	if len(provider.updated) != 1 {
		t.Errorf("expected 1 update call with force, got %d", len(provider.updated))
	}
}

func TestReconcile_DependencyOrder(t *testing.T) {
	provider := newMockProvider()
	engine := NewEngine(provider, nil)

	// Agent depends on Provider — Provider should be processed first
	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{Kind: manifest.KindAgent, Name: "bot", Spec: map[string]any{"provider": "anthropic"}},
			{Kind: manifest.KindProvider, Name: "anthropic", Spec: map[string]any{"name": "Anthropic"}},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
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

// parallelMockProvider counts concurrent Observe calls to verify parallelism.
type parallelMockProvider struct {
	mockProvider
	maxConcurrent atomic.Int64
	current       atomic.Int64
}

func (p *parallelMockProvider) Observe(ctx context.Context, kind manifest.ResourceKind, key string) (map[string]any, error) {
	cur := p.current.Add(1)
	if cur > p.maxConcurrent.Load() {
		p.maxConcurrent.Store(cur)
	}
	// Yield to encourage goroutine interleaving
	result, err := p.mockProvider.Observe(ctx, kind, key)
	p.current.Add(-1)
	return result, err
}

func TestReconcile_ExecuteInjectsWriteOnlyHash(t *testing.T) {
	provider := newMockProvider()
	engine := NewEngine(provider, nil)

	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{
				Kind: manifest.KindAgent,
				Name: "bot",
				Spec: map[string]any{
					"model":        "gpt-4",
					"contextFiles": []any{map[string]any{"IDENTITY.md": "content"}},
				},
			},
		},
	}

	// Execute (not dry-run) to verify hash is injected into provider spec
	_, result := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: false})
	if result.Applied != 1 {
		t.Fatalf("expected 1 applied, got %d", result.Applied)
	}
	hash, ok := provider.lastSpec["writeOnlyHash"].(string)
	if !ok || hash == "" {
		t.Error("expected writeOnlyHash injected into spec sent to provider")
	}
	if len(hash) != 64 {
		t.Errorf("expected 64-char SHA-256 hex, got %d chars", len(hash))
	}
}

func TestReconcile_WriteOnlyHashDriftDetected(t *testing.T) {
	provider := newMockProvider()
	// Agent exists in GoClaw with a stale hash (simulating write-only field change)
	provider.observed["Agent/bot"] = map[string]any{"model": "gpt-4", "writeOnlyHash": "stale-hash"}

	engine := NewEngine(provider, nil)
	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{
				Kind: manifest.KindAgent,
				Name: "bot",
				Spec: map[string]any{
					"model":        "gpt-4",
					"contextFiles": []any{map[string]any{"IDENTITY.md": "I am a bot"}},
				},
			},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
	if plan.Updates != 1 {
		t.Errorf("expected 1 update (hash mismatch), got updates=%d noops=%d", plan.Updates, plan.Noops)
	}
	// Verify the diff contains writeOnlyHash
	for _, ch := range plan.Changes {
		if ch.Action == ActionUpdate {
			if _, ok := ch.Diff["writeOnlyHash"]; !ok {
				t.Error("expected writeOnlyHash in diff")
			}
		}
	}
}

func TestReconcile_WriteOnlyHashMissingIsNoop(t *testing.T) {
	// Resource exists but GoClaw doesn't store/return writeOnlyHash.
	// Without an observed hash, comparison is skipped to avoid permanent update drift.
	provider := newMockProvider()
	provider.observed["Agent/bot"] = map[string]any{"model": "gpt-4"} // no writeOnlyHash

	engine := NewEngine(provider, nil)
	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{
				Kind: manifest.KindAgent,
				Name: "bot",
				Spec: map[string]any{
					"model":        "gpt-4",
					"contextFiles": []any{map[string]any{"IDENTITY.md": "I am a bot"}},
				},
			},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
	if plan.Noops != 1 {
		t.Errorf("expected noop (no observed hash to compare), got noops=%d updates=%d", plan.Noops, plan.Updates)
	}
}

func TestReconcile_WriteOnlyHashMatchesNoop(t *testing.T) {
	// Compute expected hash for the spec
	spec := map[string]any{
		"model":        "gpt-4",
		"contextFiles": []any{map[string]any{"IDENTITY.md": "I am a bot"}},
	}
	woFields := manifest.WriteOnlyFields(manifest.KindAgent)
	expectedHash := HashWriteOnlyFields(spec, woFields, nil)

	provider := newMockProvider()
	provider.observed["Agent/bot"] = map[string]any{
		"model":        "gpt-4",
		"writeOnlyHash": expectedHash,
	}

	engine := NewEngine(provider, nil)
	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{Kind: manifest.KindAgent, Name: "bot", Spec: spec},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
	if plan.Noops != 1 {
		t.Errorf("expected 1 noop (hash matches), got noops=%d updates=%d", plan.Noops, plan.Updates)
	}
}

func TestReconcile_WriteOnlyHashIgnoreAnnotation(t *testing.T) {
	provider := newMockProvider()
	// Agent exists with stale hash — normally would trigger update, but annotation disables it
	provider.observed["Agent/bot"] = map[string]any{"model": "gpt-4", "writeOnlyHash": "stale-hash"}

	engine := NewEngine(provider, nil)
	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{
				Kind: manifest.KindAgent,
				Name: "bot",
				Annotations: map[string]string{
					manifest.AnnotationIgnoreWriteOnly: "true",
				},
				Spec: map[string]any{
					"model":        "gpt-4",
					"contextFiles": []any{map[string]any{"IDENTITY.md": "content"}},
				},
			},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
	if plan.Noops != 1 {
		t.Errorf("expected noop when ignore-write-only annotation set, got noops=%d updates=%d", plan.Noops, plan.Updates)
	}
}

func TestReconcile_SyncPolicyIgnore(t *testing.T) {
	provider := newMockProvider()
	// Resource doesn't exist — normally would trigger create

	engine := NewEngine(provider, nil)
	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{
				Kind: manifest.KindProvider,
				Name: "legacy",
				Annotations: map[string]string{
					manifest.AnnotationSyncPolicy: manifest.SyncPolicyIgnore,
				},
				Spec: map[string]any{"displayName": "Legacy"},
			},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
	if plan.Noops != 1 {
		t.Errorf("expected noop with sync-policy=Ignore, got noops=%d creates=%d", plan.Noops, plan.Creates)
	}
}

func TestReconcile_IgnoreFieldsAnnotation(t *testing.T) {
	// Spec has contextFiles and systemPrompt, but systemPrompt is ignored
	spec := map[string]any{
		"model":        "gpt-4",
		"contextFiles": []any{map[string]any{"IDENTITY.md": "content"}},
		"systemPrompt": "Be helpful",
	}
	woFields := manifest.WriteOnlyFields(manifest.KindAgent)
	// Hash computed with systemPrompt excluded
	hashWithIgnore := HashWriteOnlyFields(spec, woFields, []string{"systemPrompt"})

	provider := newMockProvider()
	provider.observed["Agent/bot"] = map[string]any{
		"model":         "gpt-4",
		"writeOnlyHash": hashWithIgnore,
	}

	engine := NewEngine(provider, nil)
	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{
				Kind: manifest.KindAgent,
				Name: "bot",
				Annotations: map[string]string{
					manifest.AnnotationIgnoreFields: "systemPrompt",
				},
				Spec: spec,
			},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
	if plan.Noops != 1 {
		t.Errorf("expected noop with ignore-fields matching, got noops=%d updates=%d", plan.Noops, plan.Updates)
	}
}

func TestReconcile_ParallelProducesSameResultAsSequential(t *testing.T) {
	// Build a manifest with multiple providers (same kind → eligible for parallel within kind)
	resources := []manifest.Resource{
		{Kind: manifest.KindProvider, Name: "openai", Spec: map[string]any{"name": "OpenAI"}},
		{Kind: manifest.KindProvider, Name: "anthropic", Spec: map[string]any{"name": "Anthropic"}},
		{Kind: manifest.KindProvider, Name: "groq", Spec: map[string]any{"name": "Groq"}},
	}

	seqProvider := newMockProvider()
	seqEngine := NewEngine(seqProvider, nil)
	seqPlan, _ := seqEngine.Reconcile(context.Background(), &manifest.Manifest{Resources: resources}, ReconcileOpts{DryRun: true})

	parProvider := &parallelMockProvider{}
	parProvider.observed = make(map[string]map[string]any)
	parEngine := NewEngine(parProvider, nil)
	parPlan, _ := parEngine.Reconcile(context.Background(), &manifest.Manifest{Resources: resources}, ReconcileOpts{DryRun: true, Concurrency: 3})

	if seqPlan.Creates != parPlan.Creates {
		t.Errorf("creates mismatch: sequential=%d parallel=%d", seqPlan.Creates, parPlan.Creates)
	}
	if seqPlan.Noops != parPlan.Noops {
		t.Errorf("noops mismatch: sequential=%d parallel=%d", seqPlan.Noops, parPlan.Noops)
	}
	if len(seqPlan.Changes) != len(parPlan.Changes) {
		t.Errorf("changes count mismatch: sequential=%d parallel=%d", len(seqPlan.Changes), len(parPlan.Changes))
	}
}

// newTestLogger returns a slog.Logger backed by a buffer so tests can inspect output.
func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestWarnBlindNoop_EmitsWarnWhenBlindFieldPresent(t *testing.T) {
	// MCPCredentials.credentials is write-only (encrypted, not returned by list API).
	// Noop with credentials present in spec → emit WARN.
	provider := newMockProvider()
	provider.observed["MCPCredentials/misa"] = map[string]any{"name": "misa"}

	var buf bytes.Buffer
	engine := NewEngine(provider, newTestLogger(&buf))

	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{
				Kind: manifest.KindMCPCredentials,
				Name: "misa",
				Spec: map[string]any{
					"name":        "misa",
					"credentials": map[string]any{"token": "secret-xyz"},
				},
			},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
	if plan.Noops != 1 {
		t.Fatalf("expected 1 noop, got noops=%d updates=%d", plan.Noops, plan.Updates)
	}

	out := buf.String()
	if !strings.Contains(out, "no-op may hide drift") {
		t.Errorf("expected blind-noop warning in log output, got:\n%s", out)
	}
	if !strings.Contains(out, "credentials") {
		t.Errorf("expected 'credentials' in warning log, got:\n%s", out)
	}
}

func TestWarnBlindNoop_NoWarnWhenBlindFieldAbsent(t *testing.T) {
	// MCPCredentials without credentials in spec — noop should NOT warn.
	provider := newMockProvider()
	provider.observed["MCPCredentials/misa"] = map[string]any{"name": "misa"}

	var buf bytes.Buffer
	engine := NewEngine(provider, newTestLogger(&buf))

	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{
				Kind: manifest.KindMCPCredentials,
				Name: "misa",
				Spec: map[string]any{"name": "misa"},
			},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
	if plan.Noops != 1 {
		t.Fatalf("expected 1 noop, got noops=%d", plan.Noops)
	}

	out := buf.String()
	if strings.Contains(out, "no-op may hide drift") {
		t.Errorf("unexpected blind-noop warning when blind field absent:\n%s", out)
	}
}

func TestWarnBlindNoop_NoWarnWhenBlindFieldEmptyMap(t *testing.T) {
	// MCPCredentials.credentials present but empty — should NOT warn (trivial value).
	provider := newMockProvider()
	provider.observed["MCPCredentials/misa"] = map[string]any{"name": "misa"}

	var buf bytes.Buffer
	engine := NewEngine(provider, newTestLogger(&buf))

	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{
				Kind: manifest.KindMCPCredentials,
				Name: "misa",
				Spec: map[string]any{
					"name":        "misa",
					"credentials": map[string]any{},
				},
			},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
	if plan.Noops != 1 {
		t.Fatalf("expected 1 noop, got noops=%d", plan.Noops)
	}

	out := buf.String()
	if strings.Contains(out, "no-op may hide drift") {
		t.Errorf("unexpected blind-noop warning for empty map:\n%s", out)
	}
}

func TestWarnBlindNoop_NoWarnOnCreate(t *testing.T) {
	// Resource doesn't exist — action is create, not noop; should not warn
	provider := newMockProvider()

	var buf bytes.Buffer
	engine := NewEngine(provider, newTestLogger(&buf))

	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{
				Kind: manifest.KindBuiltinToolConfig,
				Name: "create-image",
				Spec: map[string]any{
					"enabled": true,
					"settings": map[string]any{"providers": []any{"dashscope"}},
				},
			},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
	if plan.Creates != 1 {
		t.Fatalf("expected 1 create, got creates=%d", plan.Creates)
	}

	out := buf.String()
	if strings.Contains(out, "no-op may hide drift") {
		t.Errorf("unexpected blind-noop warning on create:\n%s", out)
	}
}

func TestWarnBlindNoop_GenericAcrossKinds(t *testing.T) {
	// Provider with apiKey (blind field) — noop should warn
	provider := newMockProvider()
	provider.observed["Provider/anthropic"] = map[string]any{"displayName": "Anthropic"}

	var buf bytes.Buffer
	engine := NewEngine(provider, newTestLogger(&buf))

	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{
				Kind: manifest.KindProvider,
				Name: "anthropic",
				Spec: map[string]any{
					"displayName": "Anthropic",
					"apiKey":      "sk-secret-key",
				},
			},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
	if plan.Noops != 1 {
		t.Fatalf("expected 1 noop, got noops=%d updates=%d", plan.Noops, plan.Updates)
	}

	out := buf.String()
	if !strings.Contains(out, "no-op may hide drift") {
		t.Errorf("expected blind-noop warning for Provider/apiKey, got:\n%s", out)
	}
	if !strings.Contains(out, "apiKey") {
		t.Errorf("expected 'apiKey' in warning log, got:\n%s", out)
	}
}

func TestWarnBlindNoop_NoWarnOnHashConfirmedNoop(t *testing.T) {
	// Agent with contextFiles (a blind field) AND writeOnlyHash echoed by server
	// → hash mechanism conclusively proved no drift → must NOT warn.
	// Regression guard: without this suppression, every Agent reconcile would spam
	// the warning every loop, defeating the PR's purpose.
	spec := map[string]any{
		"model":        "gpt-4",
		"contextFiles": []any{map[string]any{"IDENTITY.md": "I am a bot"}},
	}
	woFields := manifest.WriteOnlyFields(manifest.KindAgent)
	expectedHash := HashWriteOnlyFields(spec, woFields, nil)

	provider := newMockProvider()
	provider.observed["Agent/bot"] = map[string]any{
		"model":         "gpt-4",
		"writeOnlyHash": expectedHash,
	}

	var buf bytes.Buffer
	engine := NewEngine(provider, newTestLogger(&buf))
	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{Kind: manifest.KindAgent, Name: "bot", Spec: spec},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
	if plan.Noops != 1 {
		t.Fatalf("expected 1 noop (hash matches), got noops=%d updates=%d", plan.Noops, plan.Updates)
	}

	out := buf.String()
	if strings.Contains(out, "no-op may hide drift") {
		t.Errorf("unexpected blind-noop warning when hash mechanism proved no drift:\n%s", out)
	}
}

func TestWarnBlindNoop_NoWarnWhenBlindFieldEmptyString(t *testing.T) {
	// Provider with apiKey: "" — empty-string blind field → treat as trivial, no warn.
	provider := newMockProvider()
	provider.observed["Provider/anthropic"] = map[string]any{"displayName": "Anthropic"}

	var buf bytes.Buffer
	engine := NewEngine(provider, newTestLogger(&buf))

	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{
				Kind: manifest.KindProvider,
				Name: "anthropic",
				Spec: map[string]any{
					"displayName": "Anthropic",
					"apiKey":      "",
				},
			},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
	if plan.Noops != 1 {
		t.Fatalf("expected 1 noop, got noops=%d", plan.Noops)
	}

	out := buf.String()
	if strings.Contains(out, "no-op may hide drift") {
		t.Errorf("unexpected blind-noop warning for empty-string apiKey:\n%s", out)
	}
}

func TestWarnBlindNoop_NoWarnOnForce(t *testing.T) {
	// Force flag turns a would-be-noop into an update — warn must not fire.
	provider := newMockProvider()
	provider.observed["BuiltinToolConfig/create-image"] = map[string]any{"enabled": true}

	var buf bytes.Buffer
	engine := NewEngine(provider, newTestLogger(&buf))

	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{
				Kind: manifest.KindBuiltinToolConfig,
				Name: "create-image",
				Spec: map[string]any{
					"enabled":  true,
					"settings": map[string]any{"providers": []any{"dashscope"}},
				},
			},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true, Force: true})
	if plan.Updates != 1 {
		t.Fatalf("expected 1 forced update, got updates=%d noops=%d", plan.Updates, plan.Noops)
	}

	out := buf.String()
	if strings.Contains(out, "no-op may hide drift") {
		t.Errorf("unexpected blind-noop warning on forced update:\n%s", out)
	}
}
func TestReconcile_BuiltinToolConfig_SettingsDriftDetected(t *testing.T) {
	provider := newMockProvider()
	// Observed state: dashscope is first in the chain (what's on the cluster)
	provider.observed["BuiltinToolConfig/create-image"] = map[string]any{
		"enabled": true,
		"settings": map[string]any{
			"providers": []any{"dashscope", "openai"},
		},
	}

	engine := NewEngine(provider, nil)
	// Desired state: codex-cnb is now first (what the user wants)
	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{
				Kind: manifest.KindBuiltinToolConfig,
				Name: "create-image",
				Spec: map[string]any{
					"enabled": true,
					"settings": map[string]any{
						"providers": []any{"codex-cnb", "dashscope"},
					},
				},
			},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
	if plan.Updates != 1 {
		t.Errorf("expected 1 update when provider chain changes, got updates=%d noops=%d", plan.Updates, plan.Noops)
	}
	if plan.Noops != 0 {
		t.Errorf("expected 0 noops, got %d (settings drift silently ignored)", plan.Noops)
	}
}

func TestReconcile_BuiltinToolConfig_SettingsNoSpuriousDrift(t *testing.T) {
	// No spurious drift when observed and desired settings are identical.
	provider := newMockProvider()
	provider.observed["BuiltinToolConfig/create-image"] = map[string]any{
		"enabled": true,
		"settings": map[string]any{
			"providers": []any{"dashscope", "openai"},
		},
	}

	engine := NewEngine(provider, nil)
	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{
				Kind: manifest.KindBuiltinToolConfig,
				Name: "create-image",
				Spec: map[string]any{
					"enabled": true,
					"settings": map[string]any{
						"providers": []any{"dashscope", "openai"},
					},
				},
			},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
	if plan.Noops != 1 {
		t.Errorf("expected 1 noop for identical settings, got noops=%d updates=%d", plan.Noops, plan.Updates)
	}
}

func TestReconcile_BuiltinToolConfig_BackwardCompat_NoSettings(t *testing.T) {
	// Backward compat: existing tenants with only enabled (no settings) still reconcile correctly.
	provider := newMockProvider()
	provider.observed["BuiltinToolConfig/exec"] = map[string]any{"enabled": true}

	engine := NewEngine(provider, nil)
	m := &manifest.Manifest{
		Resources: []manifest.Resource{
			{
				Kind: manifest.KindBuiltinToolConfig,
				Name: "exec",
				Spec: map[string]any{"enabled": true},
			},
		},
	}

	plan, _ := engine.Reconcile(context.Background(), m, ReconcileOpts{DryRun: true})
	if plan.Noops != 1 {
		t.Errorf("expected noop for enabled-only BuiltinToolConfig, got noops=%d updates=%d", plan.Noops, plan.Updates)
	}
}
