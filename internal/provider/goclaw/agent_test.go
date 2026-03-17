package goclaw

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

func TestAgent_Observe_Found(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/agents" {
			json.NewEncoder(w).Encode(map[string]any{
				"agents": []map[string]any{
					{"id": "a1", "agent_key": "my-agent", "display_name": "My Agent", "provider_name": "openai"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	result, err := p.Observe(manifest.KindAgent, "my-agent")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["displayName"] != "My Agent" {
		t.Errorf("expected displayName=My Agent, got %v", result["displayName"])
	}
	if result["providerName"] != "openai" {
		t.Errorf("expected providerName=openai, got %v", result["providerName"])
	}
}

func TestAgent_Observe_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"agents": []map[string]any{}})
	}))
	defer cleanup()

	result, err := p.Observe(manifest.KindAgent, "ghost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil for not-found, got %v", result)
	}
}

func TestAgent_Observe_ServerError(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer cleanup()

	_, err := p.Observe(manifest.KindAgent, "x")
	if err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestAgent_Create(t *testing.T) {
	var received map[string]any

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/agents" {
			json.NewDecoder(r.Body).Decode(&received)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(received)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	err := p.Create(manifest.KindAgent, "bot-one", map[string]any{
		"displayName":  "Bot One",
		"providerName": "openai",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if received["agent_key"] != "bot-one" {
		t.Errorf("expected agent_key=bot-one, got %v", received["agent_key"])
	}
	if received["display_name"] != "Bot One" {
		t.Errorf("expected display_name=Bot One, got %v", received["display_name"])
	}
}

func TestAgent_Update(t *testing.T) {
	var putBody map[string]any

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents":
			json.NewEncoder(w).Encode(map[string]any{
				"agents": []map[string]any{
					{"id": "uuid-bot", "agent_key": "bot-one", "display_name": "Old"},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/agents/uuid-bot":
			json.NewDecoder(r.Body).Decode(&putBody)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(putBody)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	err := p.Update(manifest.KindAgent, "bot-one", map[string]any{"displayName": "New"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if putBody["display_name"] != "New" {
		t.Errorf("expected display_name=New, got %v", putBody["display_name"])
	}
}

func TestAgent_Delete(t *testing.T) {
	deleted := false

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents":
			json.NewEncoder(w).Encode(map[string]any{
				"agents": []map[string]any{
					{"id": "uuid-bot", "agent_key": "bot-one"},
				},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/agents/uuid-bot":
			deleted = true
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	if err := p.Delete(manifest.KindAgent, "bot-one"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !deleted {
		t.Error("expected DELETE /v1/agents/uuid-bot to be called")
	}
}

func TestAgent_Delete_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"agents": []map[string]any{}})
	}))
	defer cleanup()

	if err := p.Delete(manifest.KindAgent, "ghost"); err != nil {
		t.Fatalf("idempotent delete should not error: %v", err)
	}
}

func TestAgent_ListAll(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"agents": []map[string]any{
				{"agent_key": "a1", "created_by": "gcplane"},
				{"agent_key": "a2", "created_by": "ui"},
			},
		})
	}))
	defer cleanup()

	infos, err := p.ListAll(manifest.KindAgent)
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2, got %d", len(infos))
	}
	if infos[0].Name != "a1" {
		t.Errorf("expected a1, got %s", infos[0].Name)
	}
	if infos[1].CreatedBy != "ui" {
		t.Errorf("expected ui, got %s", infos[1].CreatedBy)
	}
}
