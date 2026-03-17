package server

import (
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
)

func newTestServer() *Server {
	tracker := controller.NewStatusTracker()
	ctrl := controller.New(controller.Config{
		Tracker:  tracker,
		Interval: time.Second,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return &Server{
		tracker:    tracker,
		controller: ctrl,
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
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

func TestHandleSync_Trigger(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest("POST", "/api/v1/sync", nil)
	w := httptest.NewRecorder()
	s.handleSync(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
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
