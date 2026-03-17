package goclaw

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

// newTestServer creates an httptest server with the given handler and returns
// a Provider wired to it plus a cleanup function.
func newTestServer(t *testing.T, h http.HandlerFunc) (*Provider, func()) {
	t.Helper()
	srv := httptest.NewServer(h)
	return New(srv.URL, "test-token"), srv.Close
}

// TestProvider_Observe_Provider verifies that observeProvider returns a
// camelCase-translated map when the name matches.
func TestProvider_Observe_Provider(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/providers" && r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]any{
				"providers": []map[string]any{
					{
						"id":            "uuid-abc",
						"name":          "test-provider",
						"display_name":  "Test",
						"provider_type": "openrouter",
						"api_key":       "***",
						"enabled":       true,
					},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	result, err := p.Observe(manifest.KindProvider, "test-provider")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["providerType"] != "openrouter" {
		t.Errorf("expected providerType=openrouter, got %v", result["providerType"])
	}
	if result["displayName"] != "Test" {
		t.Errorf("expected displayName=Test, got %v", result["displayName"])
	}
}

// TestProvider_Observe_NotFound verifies that observeProvider returns nil when
// no provider matches the requested name.
func TestProvider_Observe_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"providers": []map[string]any{},
		})
	}))
	defer cleanup()

	result, err := p.Observe(manifest.KindProvider, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil for not-found, got %v", result)
	}
}

// TestProvider_Create_Provider verifies that createProvider sends a snake_case
// body with the resource name injected.
func TestProvider_Create_Provider(t *testing.T) {
	var received map[string]any

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/providers" {
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(received)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	err := p.Create(manifest.KindProvider, "new-provider", map[string]any{
		"displayName":  "New",
		"providerType": "openrouter",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if received["display_name"] != "New" {
		t.Errorf("expected snake_case display_name=New, got %v", received)
	}
	if received["provider_type"] != "openrouter" {
		t.Errorf("expected provider_type=openrouter, got %v", received)
	}
	if received["name"] != "new-provider" {
		t.Errorf("expected name=new-provider, got %v", received["name"])
	}
}

// TestProvider_Delete_Provider verifies that deleteProvider issues a DELETE
// request to the correct path using the UUID from the observe response.
func TestProvider_Delete_Provider(t *testing.T) {
	deleted := false

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/providers":
			json.NewEncoder(w).Encode(map[string]any{
				"providers": []map[string]any{
					{"id": "uuid-123", "name": "test", "provider_type": "openrouter", "api_key": "***"},
				},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/providers/uuid-123":
			deleted = true
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	err := p.Delete(manifest.KindProvider, "test")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !deleted {
		t.Error("expected DELETE /v1/providers/uuid-123 to be called")
	}
}

// TestProvider_Delete_NotFound verifies that deleteProvider is idempotent when
// the provider does not exist.
func TestProvider_Delete_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"providers": []map[string]any{},
		})
	}))
	defer cleanup()

	err := p.Delete(manifest.KindProvider, "ghost")
	if err != nil {
		t.Fatalf("expected no error for not-found delete, got: %v", err)
	}
}

// TestProvider_ListAll_Provider verifies that listAllProviders maps every
// entry in the API response to a ResourceInfo with the correct CreatedBy.
func TestProvider_ListAll_Provider(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"providers": []map[string]any{
				{"name": "p1", "created_by": "gcplane"},
				{"name": "p2", "created_by": "ui"},
			},
		})
	}))
	defer cleanup()

	infos, err := p.ListAll(manifest.KindProvider)
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 infos, got %d", len(infos))
	}
	if infos[0].Name != "p1" {
		t.Errorf("expected name=p1, got %s", infos[0].Name)
	}
	if infos[0].CreatedBy != "gcplane" {
		t.Errorf("expected createdBy=gcplane, got %s", infos[0].CreatedBy)
	}
	if infos[1].CreatedBy != "ui" {
		t.Errorf("expected createdBy=ui, got %s", infos[1].CreatedBy)
	}
}

// TestProvider_Observe_UnknownKind verifies that an unsupported kind returns an error.
func TestProvider_Observe_UnknownKind(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	_, err := p.Observe(manifest.ResourceKind("Unknown"), "x")
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
}

// TestProvider_Create_UnknownKind verifies that an unsupported kind returns an error.
func TestProvider_Create_UnknownKind(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	err := p.Create(manifest.ResourceKind("Unknown"), "x", nil)
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
}
