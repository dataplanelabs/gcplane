package goclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

func TestBuiltinToolConfig_Observe_WithTenantConfig(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/tools/builtin" {
			json.NewEncoder(w).Encode(map[string]any{
				"tools": []map[string]any{
					{"name": "exec", "enabled": true, "tenant_enabled": false},
					{"name": "web_fetch", "enabled": true},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindBuiltinToolConfig, "exec")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result for tool with tenant config")
	}
	if result["enabled"] != false {
		t.Errorf("expected enabled=false (tenant override), got %v", result["enabled"])
	}
}

func TestBuiltinToolConfig_Observe_NoTenantConfig(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/tools/builtin" {
			json.NewEncoder(w).Encode(map[string]any{
				"tools": []map[string]any{
					{"name": "web_fetch", "enabled": true},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindBuiltinToolConfig, "web-fetch")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil for tool without tenant config, got %v", result)
	}
}

func TestBuiltinToolConfig_Observe_ToolNotFound(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/tools/builtin" {
			json.NewEncoder(w).Encode(map[string]any{
				"tools": []map[string]any{
					{"name": "exec", "enabled": true},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindBuiltinToolConfig, "nonexistent")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil for unknown tool, got %v", result)
	}
}

func TestBuiltinToolConfig_Create(t *testing.T) {
	var received map[string]any

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/v1/tools/builtin/web_fetch/tenant-config" {
			json.NewDecoder(r.Body).Decode(&received)
			json.NewEncoder(w).Encode(map[string]any{"ok": true})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	err := p.Create(context.Background(), manifest.KindBuiltinToolConfig, "web-fetch", map[string]any{
		"enabled": true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if received["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", received["enabled"])
	}
}

func TestBuiltinToolConfig_Update(t *testing.T) {
	var received map[string]any

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/v1/tools/builtin/exec/tenant-config" {
			json.NewDecoder(r.Body).Decode(&received)
			json.NewEncoder(w).Encode(map[string]any{"ok": true})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	err := p.Update(context.Background(), manifest.KindBuiltinToolConfig, "exec", map[string]any{
		"enabled": false,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if received["enabled"] != false {
		t.Errorf("expected enabled=false, got %v", received["enabled"])
	}
}

func TestBuiltinToolConfig_Delete(t *testing.T) {
	deleted := false

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && r.URL.Path == "/v1/tools/builtin/read_file/tenant-config" {
			deleted = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	if err := p.Delete(context.Background(), manifest.KindBuiltinToolConfig, "read-file"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !deleted {
		t.Error("expected DELETE to be called")
	}
}

func TestToolName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"exec", "exec"},
		{"web-fetch", "web_fetch"},
		{"read-file", "read_file"},
	}
	for _, tc := range tests {
		if got := toolName(tc.input); got != tc.want {
			t.Errorf("toolName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
