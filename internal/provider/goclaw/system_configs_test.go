package goclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

func TestSystemConfig_Observe_Found(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/system-configs/tts.provider" {
			json.NewEncoder(w).Encode(map[string]any{
				"key": "tts.provider", "value": "openai",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindSystemConfig, "tts.provider")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["value"] != "openai" {
		t.Errorf("expected value=openai, got %v", result["value"])
	}
}

func TestSystemConfig_Observe_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindSystemConfig, "missing.key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestSystemConfig_Create(t *testing.T) {
	var received map[string]any

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/v1/system-configs/tts.provider" {
			json.NewDecoder(r.Body).Decode(&received)
			json.NewEncoder(w).Encode(map[string]any{"key": "tts.provider", "value": received["value"]})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	err := p.Create(context.Background(), manifest.KindSystemConfig, "tts.provider", map[string]any{
		"value": "openai",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if received["value"] != "openai" {
		t.Errorf("expected value=openai, got %v", received["value"])
	}
}

func TestSystemConfig_Update(t *testing.T) {
	var received map[string]any

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/v1/system-configs/tts.auto" {
			json.NewDecoder(r.Body).Decode(&received)
			json.NewEncoder(w).Encode(map[string]any{"key": "tts.auto", "value": received["value"]})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	err := p.Update(context.Background(), manifest.KindSystemConfig, "tts.auto", map[string]any{
		"value": "on",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if received["value"] != "on" {
		t.Errorf("expected value=on, got %v", received["value"])
	}
}

func TestSystemConfig_Delete(t *testing.T) {
	deleted := false

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && r.URL.Path == "/v1/system-configs/tts.provider" {
			deleted = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	if err := p.Delete(context.Background(), manifest.KindSystemConfig, "tts.provider"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !deleted {
		t.Error("expected DELETE /v1/system-configs/tts.provider to be called")
	}
}

func TestSysConfigPath(t *testing.T) {
	// Dots and underscores are preserved (common in system config keys)
	if got := sysConfigPath("tts.provider"); got != "/v1/system-configs/tts.provider" {
		t.Errorf("expected dots preserved, got %s", got)
	}
	if got := sysConfigPath("tts.max_length"); got != "/v1/system-configs/tts.max_length" {
		t.Errorf("expected underscores preserved, got %s", got)
	}
	// Slashes are escaped to prevent path traversal
	if got := sysConfigPath("../etc/passwd"); got == "/v1/system-configs/../etc/passwd" {
		t.Error("path traversal not escaped")
	}
}

func TestSystemConfig_Delete_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	if err := p.Delete(context.Background(), manifest.KindSystemConfig, "ghost"); err != nil {
		t.Fatalf("idempotent delete should not error: %v", err)
	}
}
