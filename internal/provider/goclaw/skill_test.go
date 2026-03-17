package goclaw

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

func TestSkill_Observe_Found(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/skills" {
			json.NewEncoder(w).Encode(map[string]any{
				"skills": []map[string]any{
					{"id": "s1", "key": "web-search", "slug": "web-search", "enabled": true},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	result, err := p.Observe(manifest.KindSkill, "web-search")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", result["enabled"])
	}
}

func TestSkill_Observe_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"skills": []map[string]any{}})
	}))
	defer cleanup()

	result, err := p.Observe(manifest.KindSkill, "missing-skill")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestSkill_Update(t *testing.T) {
	var putBody map[string]any

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills":
			json.NewEncoder(w).Encode(map[string]any{
				"skills": []map[string]any{
					{"id": "s-uuid", "key": "web-search", "slug": "web-search"},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/skills/s-uuid":
			json.NewDecoder(r.Body).Decode(&putBody)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(putBody)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	err := p.Update(manifest.KindSkill, "web-search", map[string]any{"enabled": false})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if putBody["enabled"] != false {
		t.Errorf("expected enabled=false, got %v", putBody["enabled"])
	}
}

// Skills are not deletable — Delete should return nil without calling any endpoint.
func TestSkill_Delete_Noop(t *testing.T) {
	called := false
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer cleanup()

	if err := p.Delete(manifest.KindSkill, "web-search"); err != nil {
		t.Fatalf("expected no-op delete to succeed, got: %v", err)
	}
	if called {
		t.Error("expected no HTTP calls for skill delete")
	}
}

func TestSkill_ListAll(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"skills": []map[string]any{
				{"slug": "skill-a", "created_by": "gcplane"},
				{"slug": "skill-b", "created_by": "system"},
			},
		})
	}))
	defer cleanup()

	infos, err := p.ListAll(manifest.KindSkill)
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2, got %d", len(infos))
	}
	if infos[0].Name != "skill-a" {
		t.Errorf("expected skill-a, got %s", infos[0].Name)
	}
}
