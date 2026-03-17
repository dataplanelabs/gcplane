package goclaw

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

// TestProvider_Update_Provider covers the previously-uncovered updateProvider path.
func TestProvider_Update_Provider(t *testing.T) {
	var putBody map[string]any

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/providers":
			json.NewEncoder(w).Encode(map[string]any{
				"providers": []map[string]any{
					{"id": "prov-uuid", "name": "openrouter", "provider_type": "openrouter"},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/providers/prov-uuid":
			json.NewDecoder(r.Body).Decode(&putBody)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(putBody)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	err := p.Update(manifest.KindProvider, "openrouter", map[string]any{
		"displayName":  "OpenRouter",
		"providerType": "openrouter",
	})
	if err != nil {
		t.Fatalf("update provider: %v", err)
	}
	if putBody["display_name"] != "OpenRouter" {
		t.Errorf("expected display_name=OpenRouter, got %v", putBody["display_name"])
	}
	if putBody["name"] != "openrouter" {
		t.Errorf("expected name=openrouter injected, got %v", putBody["name"])
	}
}

func TestProvider_Update_Provider_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"providers": []map[string]any{}})
	}))
	defer cleanup()

	err := p.Update(manifest.KindProvider, "ghost", map[string]any{})
	if err == nil {
		t.Fatal("expected error updating non-existent provider")
	}
}

// TestProvider_Update_UnknownKind covers the default branch in Update.
func TestProvider_Update_UnknownKind(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	err := p.Update(manifest.ResourceKind("Unknown"), "x", nil)
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
}

// TestProvider_Delete_UnknownKind covers the default branch in Delete.
func TestProvider_Delete_UnknownKind(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	err := p.Delete(manifest.ResourceKind("Unknown"), "x")
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
}

// TestProvider_ListAll_UnknownKind covers the default branch in ListAll.
func TestProvider_ListAll_UnknownKind(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	_, err := p.ListAll(manifest.ResourceKind("Unknown"))
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
}

// TestProvider_Close covers the Close method.
func TestProvider_Close(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer cleanup()

	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
