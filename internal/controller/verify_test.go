package controller

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	goclaw "github.com/dataplanelabs/gcplane/internal/provider/goclaw"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
)

// --- Mock Provider with VerifyProvider ---

type mockVerifyProvider struct {
	mockProvider
	verifyResults map[string]error // name → error
}

func (p *mockVerifyProvider) VerifyProvider(_ context.Context, name string) error {
	if err, ok := p.verifyResults[name]; ok {
		return err
	}
	return nil
}

// --- Mock Notifier with ProviderVerifyNotifier ---

type mockVerifyNotifier struct {
	driftChanges     []reconciler.Change
	verifyFailures   []ProviderVerifyFailure
	verifyNotifyCalls int
}

func (n *mockVerifyNotifier) NotifyDrift(_ context.Context, changes []reconciler.Change) error {
	n.driftChanges = changes
	return nil
}

func (n *mockVerifyNotifier) NotifyProviderVerifyFailure(_ context.Context, failures []ProviderVerifyFailure) error {
	n.verifyNotifyCalls++
	n.verifyFailures = failures
	return nil
}

// --- Helpers ---

func manifestWithProviders(names ...string) *manifest.Manifest {
	m := minimalManifest()
	for _, name := range names {
		m.Resources = append(m.Resources, manifest.Resource{
			Kind: manifest.KindProvider,
			Name: name,
			Spec: map[string]any{"providerType": "openrouter"},
		})
	}
	return m
}

func newVerifyTestController(prov reconciler.ProviderInterface, notif Notifier) *Controller {
	tracker := NewStatusTracker()
	return New(Config{
		Source:         &mockSource{manifest: minimalManifest(), hash: "test"},
		Provider:       prov,
		Tracker:        tracker,
		Notifier:       notif,
		Interval:       time.Hour,
		Prune:          false,
		Logger:         slog.Default(),
		DebounceWindow: 50 * time.Millisecond,
	})
}

// --- Tests ---

func TestVerifyProviders_AllValid(t *testing.T) {
	prov := &mockVerifyProvider{verifyResults: map[string]error{}}
	notif := &mockVerifyNotifier{}
	ctrl := newVerifyTestController(prov, notif)

	m := manifestWithProviders("p1", "p2")
	ctrl.verifyProviders(context.Background(), m)

	snap := ctrl.metrics.Snapshot()
	if snap.ProviderVerifyErrors != 0 {
		t.Errorf("expected 0 verify errors, got %d", snap.ProviderVerifyErrors)
	}
	if notif.verifyNotifyCalls != 0 {
		t.Errorf("expected 0 notify calls, got %d", notif.verifyNotifyCalls)
	}
}

func TestVerifyProviders_AuthFailure_IncrementsMetric(t *testing.T) {
	prov := &mockVerifyProvider{
		verifyResults: map[string]error{
			"bad-key": goclaw.ErrUnauthorized,
		},
	}
	notif := &mockVerifyNotifier{}
	ctrl := newVerifyTestController(prov, notif)

	m := manifestWithProviders("good", "bad-key")
	ctrl.verifyProviders(context.Background(), m)

	snap := ctrl.metrics.Snapshot()
	if snap.ProviderVerifyErrors != 1 {
		t.Errorf("expected 1 verify error, got %d", snap.ProviderVerifyErrors)
	}
}

func TestVerifyProviders_AuthFailure_SendsNotification(t *testing.T) {
	prov := &mockVerifyProvider{
		verifyResults: map[string]error{
			"p1": goclaw.ErrUnauthorized,
			"p2": goclaw.ErrUnauthorized,
		},
	}
	notif := &mockVerifyNotifier{}
	ctrl := newVerifyTestController(prov, notif)

	m := manifestWithProviders("p1", "p2")
	ctrl.verifyProviders(context.Background(), m)

	if notif.verifyNotifyCalls != 1 {
		t.Fatalf("expected 1 notify call, got %d", notif.verifyNotifyCalls)
	}
	if len(notif.verifyFailures) != 2 {
		t.Fatalf("expected 2 failures in notification, got %d", len(notif.verifyFailures))
	}
	if notif.verifyFailures[0].Name != "p1" {
		t.Errorf("expected first failure name=p1, got %s", notif.verifyFailures[0].Name)
	}
}

func TestVerifyProviders_NotFound_SkipsAllProviders(t *testing.T) {
	// When verify endpoint returns 404, stop checking all providers.
	prov := &mockVerifyProvider{
		verifyResults: map[string]error{
			"p1": goclaw.ErrNotFound,
			"p2": goclaw.ErrUnauthorized, // should not be reached
		},
	}
	notif := &mockVerifyNotifier{}
	ctrl := newVerifyTestController(prov, notif)

	m := manifestWithProviders("p1", "p2")
	ctrl.verifyProviders(context.Background(), m)

	snap := ctrl.metrics.Snapshot()
	if snap.ProviderVerifyErrors != 0 {
		t.Errorf("expected 0 verify errors when endpoint not found, got %d", snap.ProviderVerifyErrors)
	}
}

func TestVerifyProviders_NetworkError_DoesNotCountAsFailure(t *testing.T) {
	prov := &mockVerifyProvider{
		verifyResults: map[string]error{
			"p1": errors.New("connection refused"),
		},
	}
	notif := &mockVerifyNotifier{}
	ctrl := newVerifyTestController(prov, notif)

	m := manifestWithProviders("p1")
	ctrl.verifyProviders(context.Background(), m)

	snap := ctrl.metrics.Snapshot()
	if snap.ProviderVerifyErrors != 0 {
		t.Errorf("expected 0 verify errors for network error, got %d", snap.ProviderVerifyErrors)
	}
}

func TestVerifyProviders_NoProviders_Noop(t *testing.T) {
	prov := &mockVerifyProvider{}
	notif := &mockVerifyNotifier{}
	ctrl := newVerifyTestController(prov, notif)

	// Manifest with no providers
	m := minimalManifest()
	m.Resources = []manifest.Resource{
		{Kind: manifest.KindAgent, Name: "agent1"},
	}
	ctrl.verifyProviders(context.Background(), m)

	if notif.verifyNotifyCalls != 0 {
		t.Errorf("expected 0 notify calls for no providers, got %d", notif.verifyNotifyCalls)
	}
}

func TestVerifyProviders_NonVerifyProvider_Skips(t *testing.T) {
	// mockProvider (without VerifyProvider method) — type assertion should fail.
	prov := &mockProvider{}
	notif := &mockVerifyNotifier{}
	ctrl := newVerifyTestController(prov, notif)

	m := manifestWithProviders("p1")
	ctrl.verifyProviders(context.Background(), m) // should not panic
}

func TestVerifyProviders_MetricsCumulative(t *testing.T) {
	prov := &mockVerifyProvider{
		verifyResults: map[string]error{
			"p1": goclaw.ErrUnauthorized,
		},
	}
	notif := &mockVerifyNotifier{}
	ctrl := newVerifyTestController(prov, notif)

	m := manifestWithProviders("p1")
	ctrl.verifyProviders(context.Background(), m)
	ctrl.verifyProviders(context.Background(), m)

	snap := ctrl.metrics.Snapshot()
	if snap.ProviderVerifyErrors != 2 {
		t.Errorf("expected cumulative 2 verify errors, got %d", snap.ProviderVerifyErrors)
	}
}
