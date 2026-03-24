package goclaw

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestTenant_Observe_Found(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/tenants" {
			json.NewEncoder(w).Encode(map[string]any{
				"tenants": []map[string]any{
					{
						"id":         "uuid-tenant-1",
						"slug":       "acme",
						"name":       "Acme Corp",
						"created_by": "gcplane",
						"tenant_id":  "parent-tenant",
						"tenant_name": "Parent Org",
					},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	result, err := p.observeTenant("acme")
	if err != nil {
		t.Fatalf("observeTenant: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["id"] != "uuid-tenant-1" {
		t.Errorf("expected id=uuid-tenant-1, got %v", result["id"])
	}
	if result["slug"] != "acme" {
		t.Errorf("expected slug=acme, got %v", result["slug"])
	}
	if result["name"] != "Acme Corp" {
		t.Errorf("expected name=Acme Corp, got %v", result["name"])
	}
	// Verify tenant fields are stripped
	if _, ok := result["tenant_id"]; ok {
		t.Error("expected tenant_id to be stripped")
	}
	if _, ok := result["tenant_name"]; ok {
		t.Error("expected tenant_name to be stripped")
	}
}

func TestTenant_Observe_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"tenants": []map[string]any{}})
	}))
	defer cleanup()

	result, err := p.observeTenant("ghost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil for not-found, got %v", result)
	}
}

func TestTenant_Observe_ServerError(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer cleanup()

	_, err := p.observeTenant("acme")
	if err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestTenant_Observe_MultipleTenantsMatchesCorrectOne(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/tenants" {
			json.NewEncoder(w).Encode(map[string]any{
				"tenants": []map[string]any{
					{"id": "uuid-1", "slug": "acme", "name": "Acme"},
					{"id": "uuid-2", "slug": "widgets", "name": "Widgets Inc"},
					{"id": "uuid-3", "slug": "gadgets", "name": "Gadgets Co"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	result, err := p.observeTenant("widgets")
	if err != nil {
		t.Fatalf("observeTenant: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["id"] != "uuid-2" {
		t.Errorf("expected id=uuid-2, got %v", result["id"])
	}
	if result["name"] != "Widgets Inc" {
		t.Errorf("expected name=Widgets Inc, got %v", result["name"])
	}
}

func TestTenant_Create(t *testing.T) {
	var received map[string]any

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/tenants" {
			json.NewDecoder(r.Body).Decode(&received)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(received)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	err := p.createTenant("new-tenant", map[string]any{
		"name": "New Tenant Corp",
	})
	if err != nil {
		t.Fatalf("createTenant: %v", err)
	}
	if received["slug"] != "new-tenant" {
		t.Errorf("expected slug=new-tenant, got %v", received["slug"])
	}
	if received["name"] != "New Tenant Corp" {
		t.Errorf("expected name=New Tenant Corp, got %v", received["name"])
	}
}

func TestTenant_Create_CamelCaseTranslation(t *testing.T) {
	var received map[string]any

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/tenants" {
			json.NewDecoder(r.Body).Decode(&received)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(received)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	err := p.createTenant("tenant-with-config", map[string]any{
		"name":        "Tenant",
		"displayName": "Display Name",
	})
	if err != nil {
		t.Fatalf("createTenant: %v", err)
	}
	if received["display_name"] != "Display Name" {
		t.Errorf("expected snake_case display_name, got %v", received)
	}
}

func TestTenant_Update(t *testing.T) {
	var putBody map[string]any

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/tenants":
			json.NewEncoder(w).Encode(map[string]any{
				"tenants": []map[string]any{
					{"id": "uuid-tenant-1", "slug": "acme", "name": "Old Name"},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/tenants/uuid-tenant-1":
			json.NewDecoder(r.Body).Decode(&putBody)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(putBody)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	err := p.updateTenant("acme", map[string]any{"name": "New Name"})
	if err != nil {
		t.Fatalf("updateTenant: %v", err)
	}
	if putBody["name"] != "New Name" {
		t.Errorf("expected name=New Name, got %v", putBody["name"])
	}
}

func TestTenant_Update_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"tenants": []map[string]any{}})
	}))
	defer cleanup()

	err := p.updateTenant("ghost", map[string]any{"name": "New"})
	if err == nil {
		t.Fatal("expected error when tenant not found for update")
	}
}

func TestTenant_Update_MissingID(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/tenants" {
			json.NewEncoder(w).Encode(map[string]any{
				"tenants": []map[string]any{
					{"slug": "acme", "name": "Acme"}, // missing id
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	err := p.updateTenant("acme", map[string]any{"name": "New"})
	if err == nil {
		t.Fatal("expected error when tenant id is missing")
	}
}

func TestTenant_Delete(t *testing.T) {
	deleted := false

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/tenants":
			json.NewEncoder(w).Encode(map[string]any{
				"tenants": []map[string]any{
					{"id": "uuid-tenant-1", "slug": "acme"},
				},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/tenants/uuid-tenant-1":
			deleted = true
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	if err := p.deleteTenant("acme"); err != nil {
		t.Fatalf("deleteTenant: %v", err)
	}
	if !deleted {
		t.Error("expected DELETE /v1/tenants/uuid-tenant-1 to be called")
	}
}

func TestTenant_Delete_NotFound_Idempotent(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"tenants": []map[string]any{}})
	}))
	defer cleanup()

	if err := p.deleteTenant("ghost"); err != nil {
		t.Fatalf("idempotent delete should not error: %v", err)
	}
}

func TestTenant_Delete_MissingID(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/tenants" {
			json.NewEncoder(w).Encode(map[string]any{
				"tenants": []map[string]any{
					{"slug": "acme", "name": "Acme"}, // missing id
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	err := p.deleteTenant("acme")
	if err == nil {
		t.Fatal("expected error when tenant id is missing for delete")
	}
}
