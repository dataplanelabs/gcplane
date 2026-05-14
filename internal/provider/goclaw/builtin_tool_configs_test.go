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

func TestBuiltinToolConfig_Observe_WithTenantSettings(t *testing.T) {
	// When list API returns tenant_settings, observe should include settings in state
	// so drift detection compares them against the desired spec.
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/tools/builtin" {
			json.NewEncoder(w).Encode(map[string]any{
				"tools": []map[string]any{
					{
						"name":           "create_image",
						"enabled":        true,
						"tenant_enabled": true,
						"tenant_settings": map[string]any{
							"providers": []any{"dashscope", "openai"},
						},
					},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindBuiltinToolConfig, "create-image")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result when tenant config exists")
	}
	settings, ok := result["settings"]
	if !ok {
		t.Fatal("expected 'settings' key in observed state when tenant_settings present")
	}
	settingsMap, ok := settings.(map[string]any)
	if !ok {
		t.Fatalf("expected settings to be map[string]any, got %T", settings)
	}
	providers, ok := settingsMap["providers"]
	if !ok {
		t.Fatal("expected 'providers' inside settings")
	}
	provList, ok := providers.([]any)
	if !ok || len(provList) == 0 {
		t.Errorf("expected non-empty providers list, got %v", providers)
	}
}

func TestBuiltinToolConfig_Observe_NoTenantSettings(t *testing.T) {
	// Backward compat: tenant config exists (enabled override) but no settings —
	// observed state should have enabled only, no settings key.
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/tools/builtin" {
			json.NewEncoder(w).Encode(map[string]any{
				"tools": []map[string]any{
					{"name": "exec", "enabled": true, "tenant_enabled": false},
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
		t.Fatal("expected non-nil result")
	}
	if _, hasSettings := result["settings"]; hasSettings {
		t.Error("expected no 'settings' key when tenant_settings absent from list response")
	}
}

func TestBuiltinToolConfig_Create_WithSettings(t *testing.T) {
	// Settings must be sent to the tenant-config endpoint (not the global PUT).
	var tenantReceived map[string]any
	globalPutCalled := false

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/v1/tools/builtin/create_image/tenant-config":
			json.NewDecoder(r.Body).Decode(&tenantReceived)
			json.NewEncoder(w).Encode(map[string]any{"ok": true})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/tools/builtin/create_image":
			globalPutCalled = true
			json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	err := p.Create(context.Background(), manifest.KindBuiltinToolConfig, "create-image", map[string]any{
		"enabled": true,
		"settings": map[string]any{
			"providers": []any{"codex-cnb", "dashscope"},
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if globalPutCalled {
		t.Error("global PUT /v1/tools/builtin/{name} must NOT be called — settings are tenant-scoped")
	}
	if tenantReceived == nil {
		t.Fatal("expected tenant-config PUT to be called")
	}
	if tenantReceived["enabled"] != true {
		t.Errorf("expected enabled=true in tenant-config body, got %v", tenantReceived["enabled"])
	}
	settings, ok := tenantReceived["settings"]
	if !ok {
		t.Fatal("expected settings in tenant-config body")
	}
	settingsMap, ok := settings.(map[string]any)
	if !ok {
		t.Fatalf("expected settings map, got %T", settings)
	}
	providers, _ := settingsMap["providers"].([]any)
	if len(providers) == 0 || providers[0] != "codex-cnb" {
		t.Errorf("expected codex-cnb first in providers, got %v", providers)
	}
}

func TestBuiltinToolConfig_SettingsNotWriteOnly(t *testing.T) {
	// settings must NOT be in the write-only list after this fix —
	// it is now observable via tenant_settings in the list API response.
	woFields := manifest.WriteOnlyFields(manifest.KindBuiltinToolConfig)
	for _, f := range woFields {
		if f == "settings" {
			t.Errorf("'settings' must not appear in write-only fields for BuiltinToolConfig after fix; got %v", woFields)
		}
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
