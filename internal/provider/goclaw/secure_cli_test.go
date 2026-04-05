package goclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

func cliHandler(items []map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/cli-credentials" {
			json.NewEncoder(w).Encode(map[string]any{"items": items})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestSecureCLI_Observe_Found(t *testing.T) {
	p, cleanup := newTestServer(t, cliHandler([]map[string]any{
		{"id": "c1", "binary_name": "kubectl", "is_global": true, "timeout_seconds": 30.0},
	}))
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindSecureCLI, "kubectl")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["binaryName"] != "kubectl" {
		t.Errorf("expected binaryName=kubectl, got %v", result["binaryName"])
	}
	if result["isGlobal"] != true {
		t.Errorf("expected isGlobal=true, got %v", result["isGlobal"])
	}
}

func TestSecureCLI_Observe_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, cliHandler([]map[string]any{}))
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindSecureCLI, "ghost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestSecureCLI_Observe_StripsInternalFields(t *testing.T) {
	p, cleanup := newTestServer(t, cliHandler([]map[string]any{
		{
			"id": "c1", "binary_name": "kubectl",
			"created_at": "2026-01-01", "created_by": "gcplane",
			"tenant_id": "t1", "env_keys": []string{"SECRET"},
		},
	}))
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindSecureCLI, "kubectl")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	// Internal fields should be stripped
	for _, f := range []string{"createdAt", "createdBy", "tenantId", "envKeys"} {
		if _, ok := result[f]; ok {
			t.Errorf("expected field %q to be stripped", f)
		}
	}
}

func TestSecureCLI_Create(t *testing.T) {
	var received map[string]any
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/cli-credentials" {
			json.NewDecoder(r.Body).Decode(&received)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(received)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	err := p.Create(context.Background(), manifest.KindSecureCLI, "kubectl", map[string]any{
		"isGlobal":       true,
		"timeoutSeconds": 30,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if received["binary_name"] != "kubectl" {
		t.Errorf("expected binary_name=kubectl, got %v", received["binary_name"])
	}
	if received["is_global"] != true {
		t.Errorf("expected is_global=true, got %v", received["is_global"])
	}
}

func TestSecureCLI_Delete(t *testing.T) {
	deleted := false
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/cli-credentials":
			json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{{"id": "c-uuid", "binary_name": "kubectl"}},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/cli-credentials/c-uuid":
			deleted = true
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	if err := p.Delete(context.Background(), manifest.KindSecureCLI, "kubectl"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !deleted {
		t.Error("expected DELETE to be called")
	}
}

func TestSecureCLI_Delete_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, cliHandler([]map[string]any{}))
	defer cleanup()

	if err := p.Delete(context.Background(), manifest.KindSecureCLI, "ghost"); err != nil {
		t.Fatalf("idempotent delete should not error: %v", err)
	}
}

func TestSecureCLI_ListAll(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"binary_name": "kubectl", "created_by": "gcplane"},
				{"binary_name": "terraform", "created_by": "ui"},
			},
		})
	}))
	defer cleanup()

	infos, err := p.ListAll(context.Background(), manifest.KindSecureCLI)
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2, got %d", len(infos))
	}
	if infos[0].Name != "kubectl" {
		t.Errorf("expected kubectl, got %s", infos[0].Name)
	}
	if infos[1].CreatedBy != "ui" {
		t.Errorf("expected created_by=ui, got %s", infos[1].CreatedBy)
	}
}
