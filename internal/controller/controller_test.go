package controller

import (
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
)

// --- Mock Source ---

type mockSource struct {
	manifest *manifest.Manifest
	hash     string
	err      error
	calls    atomic.Int64
}

func (m *mockSource) Fetch() (*manifest.Manifest, string, error) {
	m.calls.Add(1)
	return m.manifest, m.hash, m.err
}

// --- Mock Provider ---

type mockProvider struct {
	observeResult map[string]any
	observeErr    error
	createErr     error
	updateErr     error
	deleteErr     error
	listResult    []reconciler.ResourceInfo
	listErr       error
}

func (p *mockProvider) Observe(_ manifest.ResourceKind, _ string) (map[string]any, error) {
	return p.observeResult, p.observeErr
}
func (p *mockProvider) Create(_ manifest.ResourceKind, _ string, _ map[string]any) error {
	return p.createErr
}
func (p *mockProvider) Update(_ manifest.ResourceKind, _ string, _ map[string]any) error {
	return p.updateErr
}
func (p *mockProvider) Delete(_ manifest.ResourceKind, _ string) error {
	return p.deleteErr
}
func (p *mockProvider) ListAll(_ manifest.ResourceKind) ([]reconciler.ResourceInfo, error) {
	return p.listResult, p.listErr
}

// testHash is a 16-char hash safe for controller's hash[:12] slice operation.
const testHash = "aabbccddeeff0011"

// --- Helpers ---

func minimalManifest() *manifest.Manifest {
	return &manifest.Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Metadata:   manifest.Metadata{Name: "test"},
		Connection: manifest.Connection{Endpoint: "http://localhost:9999", Token: "tok"},
		Resources:  []manifest.Resource{},
	}
}

func newTestController(src *mockSource, prov reconciler.ProviderInterface) *Controller {
	tracker := NewStatusTracker()
	return New(Config{
		Source:   src,
		Provider: prov,
		Tracker:  tracker,
		Interval: time.Hour, // large interval — only explicit calls in tests
		Prune:    false,
		Logger:   slog.Default(),
	})
}

// --- Tests ---

func TestReconcileOnce_FetchError_IncrementsErrors(t *testing.T) {
	src := &mockSource{err: errors.New("fetch failed")}
	prov := &mockProvider{}
	ctrl := newTestController(src, prov)

	ctrl.reconcileOnce()

	snap := ctrl.metrics.Snapshot()
	if snap.SyncErrors != 1 {
		t.Errorf("expected SyncErrors=1, got %d", snap.SyncErrors)
	}
	if snap.SyncSuccess != 0 {
		t.Errorf("expected SyncSuccess=0, got %d", snap.SyncSuccess)
	}
}

func TestReconcileOnce_FetchError_SetsErrorCondition(t *testing.T) {
	src := &mockSource{err: errors.New("fetch failed")}
	ctrl := newTestController(src, &mockProvider{})

	ctrl.reconcileOnce()

	status := ctrl.tracker.Get()
	var found bool
	for _, c := range status.Conditions {
		if c.Type == ConditionError && c.Status == "True" {
			found = true
		}
	}
	if !found {
		t.Error("expected Error condition to be True after fetch failure")
	}
}

func TestReconcileOnce_SuccessfulReconcile_IncrementsSuccess(t *testing.T) {
	src := &mockSource{manifest: minimalManifest(), hash: testHash}
	// observeResult=nil → ActionCreate, createErr=nil → applied
	prov := &mockProvider{observeResult: nil}
	ctrl := newTestController(src, prov)

	ctrl.reconcileOnce()

	snap := ctrl.metrics.Snapshot()
	if snap.SyncSuccess != 1 {
		t.Errorf("expected SyncSuccess=1, got %d", snap.SyncSuccess)
	}
	if snap.SyncErrors != 0 {
		t.Errorf("expected SyncErrors=0, got %d", snap.SyncErrors)
	}
}

func TestReconcileOnce_HashSkip_NoSecondReconcile(t *testing.T) {
	src := &mockSource{manifest: minimalManifest(), hash: "aabbccddeeff0011"} // ≥12 chars for hash[:12] in controller
	ctrl := newTestController(src, &mockProvider{})

	ctrl.reconcileOnce() // sets lastHash
	ctrl.reconcileOnce() // should skip because hash unchanged

	if src.calls.Load() != 2 {
		t.Errorf("expected Fetch called twice, got %d", src.calls.Load())
	}
	// Only first call does real work; metrics should show 1 success
	snap := ctrl.metrics.Snapshot()
	if snap.SyncSuccess != 1 {
		t.Errorf("expected SyncSuccess=1 (second call skipped), got %d", snap.SyncSuccess)
	}
}

func TestReconcileOnce_HashChanges_RunsReconcile(t *testing.T) {
	src := &mockSource{manifest: minimalManifest(), hash: "hash-v1"}
	ctrl := newTestController(src, &mockProvider{})

	ctrl.reconcileOnce()

	src.hash = "hash-v2"
	ctrl.reconcileOnce()

	snap := ctrl.metrics.Snapshot()
	if snap.SyncSuccess != 2 {
		t.Errorf("expected 2 successes after 2 distinct hashes, got %d", snap.SyncSuccess)
	}
}

func TestReconcileOnce_EmptyHash_AlwaysRuns(t *testing.T) {
	// hash="" means skip-check disabled — every call should reconcile
	src := &mockSource{manifest: minimalManifest(), hash: ""}
	ctrl := newTestController(src, &mockProvider{})

	ctrl.reconcileOnce()
	ctrl.reconcileOnce()

	snap := ctrl.metrics.Snapshot()
	if snap.SyncSuccess < 2 {
		t.Errorf("expected at least 2 successes for empty hash, got %d", snap.SyncSuccess)
	}
}

func TestReconcileOnce_ProviderCreateError_IncrementsErrors(t *testing.T) {
	m := minimalManifest()
	m.Resources = []manifest.Resource{
		{Kind: manifest.KindProvider, Name: "test-provider", Spec: map[string]any{"type": "anthropic"}},
	}
	src := &mockSource{manifest: m, hash: testHash}
	prov := &mockProvider{
		observeResult: nil, // triggers create
		createErr:     errors.New("create failed"),
	}
	ctrl := newTestController(src, prov)

	ctrl.reconcileOnce()

	snap := ctrl.metrics.Snapshot()
	if snap.SyncErrors != 1 {
		t.Errorf("expected SyncErrors=1 on provider create error, got %d", snap.SyncErrors)
	}
}

func TestReconcileOnce_MetricsUpdated(t *testing.T) {
	src := &mockSource{manifest: minimalManifest(), hash: testHash}
	ctrl := newTestController(src, &mockProvider{})

	before := time.Now()
	ctrl.reconcileOnce()

	snap := ctrl.metrics.Snapshot()
	if snap.LastSyncTime.Before(before) {
		t.Error("expected LastSyncTime to be updated after reconcile")
	}
	if snap.SyncDuration < 0 {
		t.Error("expected non-negative SyncDuration")
	}
}

func TestReconcileOnce_SetsLastHash(t *testing.T) {
	src := &mockSource{manifest: minimalManifest(), hash: testHash}
	ctrl := newTestController(src, &mockProvider{})

	ctrl.reconcileOnce()

	if ctrl.lastHash != testHash {
		t.Errorf("expected lastHash=%q, got %q", testHash, ctrl.lastHash)
	}
}

func TestReconcileOnce_SyncedConditionOnSuccess(t *testing.T) {
	src := &mockSource{manifest: minimalManifest(), hash: testHash}
	ctrl := newTestController(src, &mockProvider{})

	ctrl.reconcileOnce()

	status := ctrl.tracker.Get()
	var synced *Condition
	for i := range status.Conditions {
		if status.Conditions[i].Type == ConditionSynced {
			synced = &status.Conditions[i]
		}
	}
	if synced == nil {
		t.Fatal("expected Synced condition to be set")
	}
	if synced.Status != "True" {
		t.Errorf("expected Synced=True, got %s", synced.Status)
	}
}

func TestTrigger_SendsToChannel(t *testing.T) {
	src := &mockSource{manifest: minimalManifest(), hash: testHash}
	ctrl := newTestController(src, &mockProvider{})

	ctrl.Trigger()

	select {
	case <-ctrl.triggerCh:
		// good
	default:
		t.Error("expected trigger channel to have a value after Trigger()")
	}
}

func TestTrigger_NonBlocking_WhenChannelFull(t *testing.T) {
	src := &mockSource{manifest: minimalManifest(), hash: testHash}
	ctrl := newTestController(src, &mockProvider{})

	ctrl.Trigger()
	ctrl.Trigger() // should not block even if channel is full
}

func TestGetMetrics_ReturnsPointer(t *testing.T) {
	src := &mockSource{manifest: minimalManifest(), hash: testHash}
	ctrl := newTestController(src, &mockProvider{})

	m := ctrl.GetMetrics()
	if m == nil {
		t.Fatal("GetMetrics returned nil")
	}
}

func TestController_Run_StopsOnDone(t *testing.T) {
	src := &mockSource{manifest: minimalManifest(), hash: testHash}
	ctrl := newTestController(src, &mockProvider{})
	ctrl.interval = 10 * time.Millisecond

	done := make(chan struct{})
	finished := make(chan struct{})

	go func() {
		ctrl.Run(done)
		close(finished)
	}()

	time.Sleep(25 * time.Millisecond)
	close(done)

	select {
	case <-finished:
		// good
	case <-time.After(500 * time.Millisecond):
		t.Error("Run did not stop after done channel closed")
	}
}

func TestController_Run_TriggerFiresImmediateReconcile(t *testing.T) {
	src := &mockSource{manifest: minimalManifest(), hash: ""}
	ctrl := newTestController(src, &mockProvider{})
	ctrl.interval = time.Hour // disable ticker

	done := make(chan struct{})
	go ctrl.Run(done)

	// Give initial reconcile time to run
	time.Sleep(10 * time.Millisecond)
	initialCalls := src.calls.Load()

	// Trigger an immediate reconcile
	ctrl.Trigger()
	time.Sleep(20 * time.Millisecond)

	close(done)
	time.Sleep(10 * time.Millisecond)

	if src.calls.Load() <= initialCalls {
		t.Errorf("expected Fetch to be called again after Trigger, calls before=%d after=%d", initialCalls, src.calls.Load())
	}
}

func TestBuildResourceStatuses_AllActions(t *testing.T) {
	plan := &reconciler.Plan{
		Changes: []reconciler.Change{
			{Kind: manifest.KindProvider, Name: "p1", Action: reconciler.ActionCreate},
			{Kind: manifest.KindProvider, Name: "p2", Action: reconciler.ActionUpdate},
			{Kind: manifest.KindProvider, Name: "p3", Action: reconciler.ActionNoop},
			{Kind: manifest.KindProvider, Name: "p4", Action: reconciler.ActionNoop, Error: "observe failed"},
		},
	}
	result := &reconciler.ApplyResult{}

	statuses := buildResourceStatuses(plan, result)

	if len(statuses) != 4 {
		t.Fatalf("expected 4 statuses, got %d", len(statuses))
	}

	byName := make(map[string]ResourceStatus)
	for _, s := range statuses {
		byName[s.Name] = s
	}

	if byName["p1"].Status != "Created" {
		t.Errorf("p1: expected Created, got %s", byName["p1"].Status)
	}
	if byName["p2"].Status != "Updated" {
		t.Errorf("p2: expected Updated, got %s", byName["p2"].Status)
	}
	if byName["p3"].Status != "InSync" {
		t.Errorf("p3: expected InSync, got %s", byName["p3"].Status)
	}
	if byName["p4"].Status != "Error" {
		t.Errorf("p4: expected Error, got %s", byName["p4"].Status)
	}
	if byName["p4"].Message == "" {
		t.Error("p4: expected non-empty message for error status")
	}
}

func TestMetrics_Snapshot_ThreadSafe(t *testing.T) {
	m := &Metrics{}
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-done:
				return
			default:
				m.mu.Lock()
				m.SyncSuccess++
				m.mu.Unlock()
			}
		}
	}()

	for i := 0; i < 100; i++ {
		_ = m.Snapshot()
	}
	close(done)
}
