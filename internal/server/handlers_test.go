package server

import (
	"io"
	"log/slog"
	"net/http/httptest"
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
