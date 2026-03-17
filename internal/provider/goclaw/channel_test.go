package goclaw

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

// buildChannelHandler returns a handler that serves channels + agents lists.
func buildChannelHandler(instances []map[string]any, agents []map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/channels/instances":
			json.NewEncoder(w).Encode(map[string]any{"instances": instances})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents":
			json.NewEncoder(w).Encode(map[string]any{"agents": agents})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestChannel_Observe_Found(t *testing.T) {
	p, cleanup := newTestServer(t, buildChannelHandler(
		[]map[string]any{
			{"id": "ch1", "name": "slack-bot", "channel_type": "slack"},
		},
		nil,
	))
	defer cleanup()

	result, err := p.Observe(manifest.KindChannel, "slack-bot")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["channelType"] != "slack" {
		t.Errorf("expected channelType=slack, got %v", result["channelType"])
	}
}

func TestChannel_Observe_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, buildChannelHandler([]map[string]any{}, nil))
	defer cleanup()

	result, err := p.Observe(manifest.KindChannel, "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestChannel_Create(t *testing.T) {
	var received map[string]any

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents":
			json.NewEncoder(w).Encode(map[string]any{
				"agents": []map[string]any{
					{"id": "agent-uuid", "agent_key": "my-bot"},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/channels/instances":
			json.NewDecoder(r.Body).Decode(&received)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(received)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	err := p.Create(manifest.KindChannel, "slack-bot", map[string]any{
		"channelType": "slack",
		"agentKey":    "my-bot",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if received["name"] != "slack-bot" {
		t.Errorf("expected name=slack-bot, got %v", received["name"])
	}
	// agentKey should be resolved to agent_id UUID
	if received["agent_id"] != "agent-uuid" {
		t.Errorf("expected agent_id=agent-uuid, got %v", received["agent_id"])
	}
	// agent_key should be removed after resolution
	if _, ok := received["agent_key"]; ok {
		t.Error("agent_key should be removed after resolving to agent_id")
	}
}

func TestChannel_Create_AgentNotFound(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/agents" {
			json.NewEncoder(w).Encode(map[string]any{"agents": []map[string]any{}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	err := p.Create(manifest.KindChannel, "slack-bot", map[string]any{
		"agentKey": "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error when agent not found")
	}
}

func TestChannel_Update(t *testing.T) {
	var putBody map[string]any

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/channels/instances":
			json.NewEncoder(w).Encode(map[string]any{
				"instances": []map[string]any{
					{"id": "ch-uuid", "name": "slack-bot", "channel_type": "slack"},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/channels/instances/ch-uuid":
			json.NewDecoder(r.Body).Decode(&putBody)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(putBody)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	err := p.Update(manifest.KindChannel, "slack-bot", map[string]any{"channelType": "telegram"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if putBody["channel_type"] != "telegram" {
		t.Errorf("expected channel_type=telegram, got %v", putBody["channel_type"])
	}
}

func TestChannel_Delete(t *testing.T) {
	deleted := false

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/channels/instances":
			json.NewEncoder(w).Encode(map[string]any{
				"instances": []map[string]any{
					{"id": "ch-uuid", "name": "slack-bot"},
				},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/channels/instances/ch-uuid":
			deleted = true
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	if err := p.Delete(manifest.KindChannel, "slack-bot"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !deleted {
		t.Error("expected DELETE /v1/channels/instances/ch-uuid to be called")
	}
}

func TestChannel_Delete_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, buildChannelHandler([]map[string]any{}, nil))
	defer cleanup()

	if err := p.Delete(manifest.KindChannel, "ghost"); err != nil {
		t.Fatalf("idempotent delete should not error: %v", err)
	}
}

func TestChannel_ListAll(t *testing.T) {
	p, cleanup := newTestServer(t, buildChannelHandler(
		[]map[string]any{
			{"name": "ch1", "created_by": "gcplane"},
			{"name": "ch2", "created_by": "ui"},
		},
		nil,
	))
	defer cleanup()

	infos, err := p.ListAll(manifest.KindChannel)
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2, got %d", len(infos))
	}
	if infos[0].Name != "ch1" {
		t.Errorf("expected ch1, got %s", infos[0].Name)
	}
}
