package goclaw

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

func TestMCPServer_Update(t *testing.T) {
	var putBody map[string]any

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/mcp/servers":
			json.NewEncoder(w).Encode(map[string]any{
				"servers": []map[string]any{
					{"id": "m-uuid", "name": "my-mcp"},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/mcp/servers/m-uuid/grants":
			json.NewEncoder(w).Encode(map[string]any{"grants": []map[string]any{}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents":
			json.NewEncoder(w).Encode(map[string]any{"agents": []map[string]any{}})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/mcp/servers/m-uuid":
			json.NewDecoder(r.Body).Decode(&putBody)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(putBody)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	err := p.Update(manifest.KindMCPServer, "my-mcp", map[string]any{"url": "http://new.local"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if putBody["url"] != "http://new.local" {
		t.Errorf("expected url=http://new.local, got %v", putBody["url"])
	}
	// grants must be stripped from update body
	if _, ok := putBody["grants"]; ok {
		t.Error("grants should be stripped from update body")
	}
}

func TestMCPServer_Update_WithGrantChanges(t *testing.T) {
	grantAdded := false
	grantRevoked := false

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/mcp/servers":
			json.NewEncoder(w).Encode(map[string]any{
				"servers": []map[string]any{{"id": "m-uuid", "name": "my-mcp"}},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/mcp/servers/m-uuid":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{})
		// grants list: currently has old-agent
		case r.Method == http.MethodGet && r.URL.Path == "/v1/mcp/servers/m-uuid/grants":
			json.NewEncoder(w).Encode(map[string]any{
				"grants": []map[string]any{{"agent_id": "old-uuid"}},
			})
		// agents list: old-uuid → old-bot, new-uuid → new-bot
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents":
			json.NewEncoder(w).Encode(map[string]any{
				"agents": []map[string]any{
					{"id": "old-uuid", "agent_key": "old-bot"},
					{"id": "new-uuid", "agent_key": "new-bot"},
				},
			})
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/grants/agent"):
			grantAdded = true
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{})
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/grants/agent/"):
			grantRevoked = true
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	// Desired: new-bot only → old-bot should be revoked, new-bot granted
	err := p.Update(manifest.KindMCPServer, "my-mcp", map[string]any{
		"url":    "http://mcp.local",
		"grants": map[string]any{"agents": []any{"new-bot"}},
	})
	if err != nil {
		t.Fatalf("update with grant changes: %v", err)
	}
	if !grantAdded {
		t.Error("expected grant add for new-bot")
	}
	if !grantRevoked {
		t.Error("expected grant revoke for old-bot")
	}
}

func TestMCPServer_Update_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"servers": []map[string]any{}})
	}))
	defer cleanup()

	err := p.Update(manifest.KindMCPServer, "ghost", map[string]any{})
	if err == nil {
		t.Fatal("expected error updating non-existent mcp server")
	}
}
