package goclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

func TestSkillConfig_Observe_WithTenantConfig(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/skills" {
			json.NewEncoder(w).Encode(map[string]any{
				"skills": []map[string]any{
					{
						"id": "uuid-1", "slug": "my-skill", "enabled": true,
						"tenant_config": map[string]any{"enabled": false},
					},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindSkillConfig, "my-skill")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result for skill with tenant config")
	}
	if result["enabled"] != false {
		t.Errorf("expected enabled=false (tenant override), got %v", result["enabled"])
	}
}

func TestSkillConfig_Observe_NoTenantConfig(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/skills" {
			json.NewEncoder(w).Encode(map[string]any{
				"skills": []map[string]any{
					{"id": "uuid-1", "slug": "my-skill", "enabled": true},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindSkillConfig, "my-skill")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil for skill without tenant config, got %v", result)
	}
}

func TestSkillConfig_Observe_SkillNotFound(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/skills" {
			json.NewEncoder(w).Encode(map[string]any{
				"skills": []map[string]any{
					{"id": "uuid-1", "slug": "other-skill", "enabled": true},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindSkillConfig, "nonexistent")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil for unknown skill, got %v", result)
	}
}

func TestSkillConfig_Create(t *testing.T) {
	var received map[string]any

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills":
			json.NewEncoder(w).Encode(map[string]any{
				"skills": []map[string]any{
					{"id": "uuid-abc", "slug": "my-skill", "enabled": true},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/skills/uuid-abc/tenant-config":
			json.NewDecoder(r.Body).Decode(&received)
			json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	err := p.Create(context.Background(), manifest.KindSkillConfig, "my-skill", map[string]any{
		"enabled": false,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if received["enabled"] != false {
		t.Errorf("expected enabled=false, got %v", received["enabled"])
	}
}

func TestSkillConfig_Delete(t *testing.T) {
	deleted := false

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills":
			json.NewEncoder(w).Encode(map[string]any{
				"skills": []map[string]any{
					{"id": "uuid-abc", "slug": "my-skill", "enabled": true},
				},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/skills/uuid-abc/tenant-config":
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	if err := p.Delete(context.Background(), manifest.KindSkillConfig, "my-skill"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !deleted {
		t.Error("expected DELETE to be called")
	}
}
