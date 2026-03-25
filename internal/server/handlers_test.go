package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dataplanelabs/gcplane/internal/controller"
	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
)

// stubSource is a minimal ManifestSource for handler tests.
type stubSource struct{}

func (s *stubSource) Fetch() (*manifest.Manifest, string, error) {
	m := &manifest.Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Metadata:   manifest.Metadata{Name: "test"},
		Connection: manifest.Connection{Endpoint: "http://localhost:9999", Token: "tok"},
		Resources:  []manifest.Resource{},
	}
	return m, "", nil
}

// stubProvider is a no-op provider for handler tests.
type stubProvider struct{}

func (p *stubProvider) Observe(_ context.Context, _ manifest.ResourceKind, _ string) (map[string]any, error) {
	return map[string]any{}, nil
}
func (p *stubProvider) Create(_ context.Context, _ manifest.ResourceKind, _ string, _ map[string]any) error {
	return nil
}
func (p *stubProvider) Update(_ context.Context, _ manifest.ResourceKind, _ string, _ map[string]any) error {
	return nil
}
func (p *stubProvider) Delete(_ context.Context, _ manifest.ResourceKind, _ string) error { return nil }
func (p *stubProvider) ListAll(_ context.Context, _ manifest.ResourceKind) ([]reconciler.ResourceInfo, error) {
	return nil, nil
}

func newTestServer() *Server {
	tracker := controller.NewStatusTracker()
	ctrl := controller.New(controller.Config{
		Tracker:        tracker,
		Interval:       time.Second,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		DebounceWindow: 10 * time.Millisecond,
	})
	return &Server{
		tracker:    tracker,
		controller: ctrl,
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// newRunnableTestServer creates a server whose controller is actively running.
// The returned stop func must be called to clean up.
func newRunnableTestServer() (*Server, func()) {
	tracker := controller.NewStatusTracker()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctrl := controller.New(controller.Config{
		Source:         &stubSource{},
		Provider:       &stubProvider{},
		Tracker:        tracker,
		Interval:       time.Hour,
		Logger:         logger,
		DebounceWindow: 10 * time.Millisecond,
	})
	done := make(chan struct{})
	go ctrl.Run(done)
	s := &Server{
		tracker:    tracker,
		controller: ctrl,
		logger:     logger,
	}
	return s, func() { close(done) }
}

func TestHealthz(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	s.handleHealthz(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestReadyz_NotSynced(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	s.handleReadyz(w, req)
	if w.Code != 503 {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestReadyz_Synced(t *testing.T) {
	s := newTestServer()
	s.tracker.SetCondition(controller.Condition{
		Type: controller.ConditionSynced, Status: "True",
	})
	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	s.handleReadyz(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestStatus(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()
	s.handleStatus(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleSync_ReturnsJSON(t *testing.T) {
	s, stop := newRunnableTestServer()
	defer stop()

	// Give the controller time for its initial reconcile (no source → fast).
	time.Sleep(20 * time.Millisecond)

	req := httptest.NewRequest("POST", "/api/v1/sync", nil)
	w := httptest.NewRecorder()
	s.handleSync(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if body == "" {
		t.Fatal("expected non-empty JSON body")
	}
	// Verify response contains expected SyncResult fields.
	for _, field := range []string{"applied", "failed", "creates", "updates", "noops"} {
		if !strings.Contains(body, field) {
			t.Errorf("expected response body to contain %q, got: %s", field, body)
		}
	}
}

func TestHandleSync_Timeout(t *testing.T) {
	// Controller with no Run loop — TriggerAndWait will time out.
	s := newTestServer()

	req := httptest.NewRequest("POST", "/api/v1/sync", nil)
	// Use a very short deadline to force timeout quickly.
	ctx, cancel := context.WithTimeout(req.Context(), 30*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	s.handleSync(w, req)
	if w.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504, got %d", w.Code)
	}
}

func TestHandleWebhook(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest("POST", "/api/v1/webhook/git", nil)
	w := httptest.NewRecorder()
	s.handleWebhook(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func newTestServerWithSecret(secret string) *Server {
	s := newTestServer()
	s.webhookSecret = secret
	return s
}

func githubSig(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWebhook_NoSecret(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest("POST", "/api/v1/webhook/git", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	s.handleWebhook(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestWebhook_GitHubValid(t *testing.T) {
	const secret = "mysecret"
	body := `{"ref":"refs/heads/main"}`
	s := newTestServerWithSecret(secret)
	req := httptest.NewRequest("POST", "/api/v1/webhook/git", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", githubSig(secret, body))
	w := httptest.NewRecorder()
	s.handleWebhook(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestWebhook_GitHubInvalid(t *testing.T) {
	s := newTestServerWithSecret("mysecret")
	body := `{"ref":"refs/heads/main"}`
	req := httptest.NewRequest("POST", "/api/v1/webhook/git", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalidsignature")
	w := httptest.NewRecorder()
	s.handleWebhook(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestWebhook_GitLabValid(t *testing.T) {
	const secret = "gitlab-token"
	s := newTestServerWithSecret(secret)
	req := httptest.NewRequest("POST", "/api/v1/webhook/git", strings.NewReader("{}"))
	req.Header.Set("X-Gitlab-Token", secret)
	w := httptest.NewRecorder()
	s.handleWebhook(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleMetrics(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	s.handleMetrics(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if body == "" {
		t.Fatal("expected non-empty metrics body")
	}
}
