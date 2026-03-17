package goclaw

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

// mcpHandler builds a handler covering all MCP + agent endpoints used by tests.
func mcpHandler(servers []map[string]any, agents []map[string]any, grants []map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/mcp/servers":
			json.NewEncoder(w).Encode(map[string]any{"servers": servers})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents":
			json.NewEncoder(w).Encode(map[string]any{"agents": agents})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/grants"):
			json.NewEncoder(w).Encode(map[string]any{"grants": grants})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestMCPServer_Observe_Found(t *testing.T) {
	p, cleanup := newTestServer(t, mcpHandler(
		[]map[string]any{{"id": "m1", "name": "my-mcp", "url": "http://mcp.local"}},
		[]map[string]any{{"id": "a1", "agent_key": "bot"}},
		[]map[string]any{{"agent_id": "a1"}},
	))
	defer cleanup()

	result, err := p.Observe(manifest.KindMCPServer, "my-mcp")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// grants should be injected
	grants, ok := result["grants"].(map[string]any)
	if !ok {
		t.Fatalf("expected grants map, got %T", result["grants"])
	}
	agents, ok := grants["agents"].([]string)
	if !ok {
		t.Fatalf("expected []string agents, got %T", grants["agents"])
	}
	if len(agents) != 1 || agents[0] != "bot" {
		t.Errorf("expected grants.agents=[bot], got %v", agents)
	}
}

func TestMCPServer_Observe_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, mcpHandler([]map[string]any{}, nil, nil))
	defer cleanup()

	result, err := p.Observe(manifest.KindMCPServer, "ghost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestMCPServer_Create_NoGrants(t *testing.T) {
	var received map[string]any

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/mcp/servers":
			json.NewDecoder(r.Body).Decode(&received)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(received)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	err := p.Create(manifest.KindMCPServer, "my-mcp", map[string]any{
		"url": "http://mcp.local",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if received["name"] != "my-mcp" {
		t.Errorf("expected name=my-mcp, got %v", received["name"])
	}
	// grants field should not be in body
	if _, ok := received["grants"]; ok {
		t.Error("grants should be stripped from create body")
	}
}

func TestMCPServer_Create_WithGrants(t *testing.T) {
	grantCalled := false

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/mcp/servers":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": "m1", "name": "my-mcp"})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/mcp/servers":
			json.NewEncoder(w).Encode(map[string]any{
				"servers": []map[string]any{{"id": "m1", "name": "my-mcp"}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/mcp/servers/m1/grants":
			json.NewEncoder(w).Encode(map[string]any{"grants": []map[string]any{}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents":
			json.NewEncoder(w).Encode(map[string]any{
				"agents": []map[string]any{{"id": "a-uuid", "agent_key": "bot"}},
			})
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/grants/agent"):
			grantCalled = true
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	err := p.Create(manifest.KindMCPServer, "my-mcp", map[string]any{
		"url":    "http://mcp.local",
		"grants": map[string]any{"agents": []any{"bot"}},
	})
	if err != nil {
		t.Fatalf("create with grants: %v", err)
	}
	if !grantCalled {
		t.Error("expected grant POST to be called")
	}
}

func TestMCPServer_Delete(t *testing.T) {
	deleted := false

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/mcp/servers":
			json.NewEncoder(w).Encode(map[string]any{
				"servers": []map[string]any{{"id": "m-uuid", "name": "my-mcp"}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/mcp/servers/m-uuid/grants":
			json.NewEncoder(w).Encode(map[string]any{"grants": []map[string]any{}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents":
			json.NewEncoder(w).Encode(map[string]any{"agents": []map[string]any{}})
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/mcp/servers/m-uuid":
			deleted = true
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	if err := p.Delete(manifest.KindMCPServer, "my-mcp"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !deleted {
		t.Error("expected DELETE /v1/mcp/servers/m-uuid to be called")
	}
}

func TestMCPServer_Delete_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, mcpHandler([]map[string]any{}, nil, nil))
	defer cleanup()

	if err := p.Delete(manifest.KindMCPServer, "ghost"); err != nil {
		t.Fatalf("idempotent delete should not error: %v", err)
	}
}

func TestMCPServer_ListAll(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"servers": []map[string]any{
				{"name": "s1", "created_by": "gcplane"},
				{"name": "s2", "created_by": "ui"},
			},
		})
	}))
	defer cleanup()

	infos, err := p.ListAll(manifest.KindMCPServer)
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2, got %d", len(infos))
	}
	if infos[0].Name != "s1" {
		t.Errorf("expected s1, got %s", infos[0].Name)
	}
}

func TestExtractGrantAgents(t *testing.T) {
	tests := []struct {
		name     string
		spec     map[string]any
		expected []string
	}{
		{
			name:     "no grants key",
			spec:     map[string]any{"url": "x"},
			expected: nil,
		},
		{
			name:     "grants with []string",
			spec:     map[string]any{"grants": map[string]any{"agents": []string{"a", "b"}}},
			expected: []string{"a", "b"},
		},
		{
			name:     "grants with []any",
			spec:     map[string]any{"grants": map[string]any{"agents": []any{"x", "y"}}},
			expected: []string{"x", "y"},
		},
		{
			name:     "grants missing agents key",
			spec:     map[string]any{"grants": map[string]any{}},
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractGrantAgents(tc.spec)
			if len(got) != len(tc.expected) {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
			for i, v := range got {
				if v != tc.expected[i] {
					t.Errorf("index %d: expected %s, got %s", i, tc.expected[i], v)
				}
			}
		})
	}
}
