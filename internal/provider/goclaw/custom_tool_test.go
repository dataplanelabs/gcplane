package goclaw

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

func TestCustomTool_Observe_Found(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/tools/custom" {
			json.NewEncoder(w).Encode(map[string]any{
				"tools": []map[string]any{
					{"id": "t1", "name": "my-tool", "tool_type": "http"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	result, err := p.Observe(manifest.KindTool, "my-tool")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["toolType"] != "http" {
		t.Errorf("expected toolType=http, got %v", result["toolType"])
	}
}

func TestCustomTool_Observe_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"tools": []map[string]any{}})
	}))
	defer cleanup()

	result, err := p.Observe(manifest.KindTool, "ghost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestCustomTool_Create(t *testing.T) {
	var received map[string]any

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/tools/custom" {
			json.NewDecoder(r.Body).Decode(&received)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(received)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	err := p.Create(manifest.KindTool, "my-tool", map[string]any{
		"toolType":    "http",
		"description": "A test tool",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if received["name"] != "my-tool" {
		t.Errorf("expected name=my-tool, got %v", received["name"])
	}
	if received["tool_type"] != "http" {
		t.Errorf("expected tool_type=http, got %v", received["tool_type"])
	}
}

func TestCustomTool_Update(t *testing.T) {
	var putBody map[string]any

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/tools/custom":
			json.NewEncoder(w).Encode(map[string]any{
				"tools": []map[string]any{
					{"id": "t-uuid", "name": "my-tool"},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/tools/custom/t-uuid":
			json.NewDecoder(r.Body).Decode(&putBody)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(putBody)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	err := p.Update(manifest.KindTool, "my-tool", map[string]any{"description": "updated"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if putBody["description"] != "updated" {
		t.Errorf("expected description=updated, got %v", putBody["description"])
	}
}

func TestCustomTool_Delete(t *testing.T) {
	deleted := false

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/tools/custom":
			json.NewEncoder(w).Encode(map[string]any{
				"tools": []map[string]any{
					{"id": "t-uuid", "name": "my-tool"},
				},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/tools/custom/t-uuid":
			deleted = true
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	if err := p.Delete(manifest.KindTool, "my-tool"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !deleted {
		t.Error("expected DELETE /v1/tools/custom/t-uuid to be called")
	}
}

func TestCustomTool_Delete_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"tools": []map[string]any{}})
	}))
	defer cleanup()

	if err := p.Delete(manifest.KindTool, "ghost"); err != nil {
		t.Fatalf("idempotent delete should not error: %v", err)
	}
}

func TestCustomTool_ListAll(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"tools": []map[string]any{
				{"name": "t1", "created_by": "gcplane"},
				{"name": "t2", "created_by": "ui"},
			},
		})
	}))
	defer cleanup()

	infos, err := p.ListAll(manifest.KindTool)
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2, got %d", len(infos))
	}
	if infos[0].Name != "t1" {
		t.Errorf("expected t1, got %s", infos[0].Name)
	}
	if infos[1].CreatedBy != "ui" {
		t.Errorf("expected ui, got %s", infos[1].CreatedBy)
	}
}
