package goclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

func TestParseGrantName_Valid(t *testing.T) {
	tests := []struct {
		name       string
		wantBinary string
		wantAgent  string
	}{
		{"kubectl--assistant", "kubectl", "assistant"},
		{"my-cli--my-agent", "my-cli", "my-agent"},
		{"a--b", "a", "b"},
	}
	for _, tc := range tests {
		binary, agent, err := parseGrantName(tc.name)
		if err != nil {
			t.Errorf("parseGrantName(%q): unexpected error: %v", tc.name, err)
			continue
		}
		if binary != tc.wantBinary || agent != tc.wantAgent {
			t.Errorf("parseGrantName(%q) = (%q, %q), want (%q, %q)",
				tc.name, binary, agent, tc.wantBinary, tc.wantAgent)
		}
	}
}

func TestParseGrantName_Invalid(t *testing.T) {
	for _, name := range []string{"noseparator", "--leading", "trailing--", ""} {
		_, _, err := parseGrantName(name)
		if err == nil {
			t.Errorf("parseGrantName(%q): expected error, got nil", name)
		}
	}
}

// grantHandler builds a handler covering cli-credentials + agents + grants endpoints.
func grantHandler(
	items []map[string]any,
	agents []map[string]any,
	grants []map[string]any,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/cli-credentials":
			json.NewEncoder(w).Encode(map[string]any{"items": items})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents":
			json.NewEncoder(w).Encode(map[string]any{"agents": agents})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/agent-grants"):
			json.NewEncoder(w).Encode(map[string]any{"grants": grants})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestSecureCLIGrant_Observe_Found(t *testing.T) {
	p, cleanup := newTestServer(t, grantHandler(
		[]map[string]any{{"id": "b1", "binary_name": "kubectl"}},
		[]map[string]any{{"id": "a1", "agent_key": "assistant"}},
		[]map[string]any{{"id": "g1", "agent_id": "a1", "binary_id": "b1", "enabled": true, "timeout_seconds": 60.0}},
	))
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindSecureCLIGrant, "kubectl--assistant")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", result["enabled"])
	}
	// Internal fields should be stripped
	for _, f := range []string{"binaryId", "agentId"} {
		if _, ok := result[f]; ok {
			t.Errorf("expected field %q to be stripped", f)
		}
	}
}

func TestSecureCLIGrant_Observe_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, grantHandler(
		[]map[string]any{{"id": "b1", "binary_name": "kubectl"}},
		[]map[string]any{{"id": "a1", "agent_key": "assistant"}},
		[]map[string]any{}, // no grants
	))
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindSecureCLIGrant, "kubectl--assistant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestSecureCLIGrant_Create(t *testing.T) {
	var received map[string]any
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/cli-credentials":
			json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{{"id": "b1", "binary_name": "kubectl"}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents":
			json.NewEncoder(w).Encode(map[string]any{
				"agents": []map[string]any{{"id": "a1", "agent_key": "assistant"}},
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/agent-grants"):
			json.NewDecoder(r.Body).Decode(&received)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(received)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	err := p.Create(context.Background(), manifest.KindSecureCLIGrant, "kubectl--assistant", map[string]any{
		"binaryName":     "kubectl",
		"agentKey":       "assistant",
		"timeoutSeconds": 60,
		"enabled":        true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if received["agent_id"] != "a1" {
		t.Errorf("expected agent_id=a1, got %v", received["agent_id"])
	}
	// Manifest-only fields should be removed from request body
	if _, ok := received["binary_name"]; ok {
		t.Error("binary_name should be stripped from request body")
	}
	if _, ok := received["agent_key"]; ok {
		t.Error("agent_key should be stripped from request body")
	}
}

func TestSecureCLIGrant_Delete_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, grantHandler(
		[]map[string]any{{"id": "b1", "binary_name": "kubectl"}},
		[]map[string]any{{"id": "a1", "agent_key": "assistant"}},
		[]map[string]any{},
	))
	defer cleanup()

	if err := p.Delete(context.Background(), manifest.KindSecureCLIGrant, "kubectl--assistant"); err != nil {
		t.Fatalf("idempotent delete should not error: %v", err)
	}
}
