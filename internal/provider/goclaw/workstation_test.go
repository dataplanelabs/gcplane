package goclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

func agentsHandlerFor(agents []map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/agents" {
			_ = json.NewEncoder(w).Encode(map[string]any{"agents": agents})
			return
		}
		http.NotFound(w, r)
	}
}

func TestWorkstation_Observe_Found(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "workstations.list", ok: true, payload: map[string]any{
			"workstations": []map[string]any{
				{
					"id":             "ws-uuid-1",
					"workstationKey": "coding-agent",
					"name":           "Coding Agent (codex)",
					"backendType":    "ssh",
					"defaultCwd":     "/workspace",
					"active":         true,
					"tenantId":       "tenant-uuid",
					"createdBy":      "gcplane",
					"createdAt":      "2026-01-01T00:00:00Z",
					"updatedAt":      "2026-01-01T00:00:00Z",
					"metadataSummary": map[string]any{"host": "ssh.example.com", "hasKey": true},
				},
			},
		}},
	}, nil)
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindWorkstation, "coding-agent")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["workstationKey"] != "coding-agent" {
		t.Errorf("workstationKey = %v, want coding-agent", result["workstationKey"])
	}
	if result["backendType"] != "ssh" {
		t.Errorf("backendType = %v, want ssh", result["backendType"])
	}
}

func TestWorkstation_Observe_NotFound(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "workstations.list", ok: true, payload: map[string]any{
			"workstations": []map[string]any{},
		}},
	}, nil)
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindWorkstation, "ghost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil for missing workstation, got %v", result)
	}
}

func TestWorkstation_Observe_StripsInternalFields(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "workstations.list", ok: true, payload: map[string]any{
			"workstations": []map[string]any{
				{
					"id":              "ws-uuid-1",
					"workstationKey":  "coding-agent",
					"name":            "Coding Agent",
					"backendType":     "ssh",
					"tenantId":        "t-uuid",
					"createdBy":       "gcplane",
					"createdAt":       "2026-01-01T00:00:00Z",
					"updatedAt":       "2026-01-01T00:00:00Z",
					"metadataSummary": map[string]any{"host": "ssh.example.com"},
				},
			},
		}},
	}, nil)
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindWorkstation, "coding-agent")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	for _, f := range workstationInternalFields {
		if _, ok := result[f]; ok {
			t.Errorf("internal field %q should be stripped from observe result", f)
		}
	}
	// Observable fields should survive.
	if result["name"] != "Coding Agent" {
		t.Errorf("name should be preserved, got %v", result["name"])
	}
}

func TestWorkstation_Create(t *testing.T) {
	agents := []map[string]any{{"id": "agent-uuid-1", "agent_key": "assistant"}}
	wsCalls := []string{}

	responses := []wsResponse{
		{method: "workstations.create", ok: true, payload: map[string]any{"workstation": map[string]any{
			"id":             "ws-new",
			"workstationKey": "coding-agent",
		}}},
		// resolve after create
		{method: "workstations.list", ok: true, payload: map[string]any{
			"workstations": []map[string]any{{"id": "ws-new", "workstationKey": "coding-agent", "createdBy": "gcplane"}},
		}},
		// allowlist
		{method: "workstations.permissions.add", ok: true, payload: map[string]any{"permission": map[string]any{"id": "p1", "pattern": "git"}}},
		{method: "workstations.permissions.add", ok: true, payload: map[string]any{"permission": map[string]any{"id": "p2", "pattern": "ls"}}},
		// agent link
		{method: "workstations.linkAgent", ok: true,
			assertParams: func(t *testing.T, params any) {
				t.Helper()
				got, _ := params.(map[string]any)
				if got["agentId"] != "agent-uuid-1" {
					t.Errorf("linkAgent agentId = %v, want agent-uuid-1", got["agentId"])
				}
				if got["workstationId"] != "ws-new" {
					t.Errorf("linkAgent workstationId = %v, want ws-new", got["workstationId"])
				}
			},
			payload: map[string]any{"linked": true}},
	}
	_ = wsCalls

	p, cleanup := newWSTestServer(t, responses, agentsHandlerFor(agents))
	defer cleanup()

	err := p.Create(context.Background(), manifest.KindWorkstation, "coding-agent", map[string]any{
		"displayName": "Coding Agent (codex)",
		"backendType": "ssh",
		"host":        "coding-agent-ssh.coding-agent.svc.cluster.local",
		"port":        22,
		"user":        "claude",
		"privateKey":  "-----BEGIN OPENSSH PRIVATE KEY-----\ntest\n-----END OPENSSH PRIVATE KEY-----",
		"defaultCwd":  "/workspace",
		"allowlist":   []any{"git", "ls"},
		"agents":      []any{"assistant"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
}

func TestWorkstation_Create_NoAllowlistNoAgents(t *testing.T) {
	responses := []wsResponse{
		{method: "workstations.create", ok: true, payload: map[string]any{"workstation": map[string]any{
			"id":             "ws-bare",
			"workstationKey": "bare-ws",
		}}},
		{method: "workstations.list", ok: true, payload: map[string]any{
			"workstations": []map[string]any{{"id": "ws-bare", "workstationKey": "bare-ws", "createdBy": "gcplane"}},
		}},
	}

	p, cleanup := newWSTestServer(t, responses, nil)
	defer cleanup()

	err := p.Create(context.Background(), manifest.KindWorkstation, "bare-ws", map[string]any{
		"displayName": "Bare Workstation",
		"backendType": "ssh",
	})
	if err != nil {
		t.Fatalf("create without allowlist/agents: %v", err)
	}
}

func TestWorkstation_Update(t *testing.T) {
	responses := []wsResponse{
		// resolve ID
		{method: "workstations.list", ok: true, payload: map[string]any{
			"workstations": []map[string]any{{"id": "ws-uuid-1", "workstationKey": "coding-agent", "createdBy": "gcplane"}},
		}},
		{method: "workstations.update", ok: true, payload: map[string]any{"id": "ws-uuid-1"}},
		// list permissions for diff
		{method: "workstations.permissions.list", ok: true, payload: map[string]any{
			"permissions": []map[string]any{
				{"id": "p1", "pattern": "git"},
				{"id": "p2", "pattern": "old-cmd"},
			},
		}},
		// add new, remove surplus
		{method: "workstations.permissions.add", ok: true, payload: map[string]any{"permission": map[string]any{"id": "p3", "pattern": "ls"}}},
		{method: "workstations.permissions.remove", ok: true, payload: map[string]any{"id": "p2"}},
	}

	p, cleanup := newWSTestServer(t, responses, nil)
	defer cleanup()

	err := p.Update(context.Background(), manifest.KindWorkstation, "coding-agent", map[string]any{
		"displayName": "Coding Agent Updated",
		"allowlist":   []any{"git", "ls"},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
}

func TestWorkstation_Delete(t *testing.T) {
	responses := []wsResponse{
		{method: "workstations.list", ok: true, payload: map[string]any{
			"workstations": []map[string]any{{"id": "ws-uuid-1", "workstationKey": "coding-agent"}},
		}},
		{method: "workstations.delete", ok: true, payload: map[string]any{"id": "ws-uuid-1"}},
	}

	p, cleanup := newWSTestServer(t, responses, nil)
	defer cleanup()

	if err := p.Delete(context.Background(), manifest.KindWorkstation, "coding-agent"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestWorkstation_Delete_NotFound(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "workstations.list", ok: true, payload: map[string]any{
			"workstations": []map[string]any{},
		}},
	}, nil)
	defer cleanup()

	if err := p.Delete(context.Background(), manifest.KindWorkstation, "ghost"); err != nil {
		t.Fatalf("idempotent delete should not error: %v", err)
	}
}

func TestWorkstation_ListAll(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "workstations.list", ok: true, payload: map[string]any{
			"workstations": []map[string]any{
				{"id": "w1", "workstationKey": "coding-agent", "createdBy": "gcplane"},
				{"id": "w2", "workstationKey": "staging-ws", "createdBy": "ui"},
			},
		}},
	}, nil)
	defer cleanup()

	infos, err := p.ListAll(context.Background(), manifest.KindWorkstation)
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 infos, got %d", len(infos))
	}
	if infos[0].Name != "coding-agent" {
		t.Errorf("infos[0].Name = %v, want coding-agent", infos[0].Name)
	}
	if infos[1].CreatedBy != "ui" {
		t.Errorf("infos[1].CreatedBy = %v, want ui", infos[1].CreatedBy)
	}
}

func TestWorkstation_BuildCreateParams(t *testing.T) {
	spec := map[string]any{
		"displayName":           "Coding Agent",
		"backendType":           "ssh",
		"host":                  "ssh.example.com",
		"port":                  22,
		"user":                  "claude",
		"privateKey":            "PRIVATEKEY",
		"knownHostsFingerprint": "SHA256:abc",
		"defaultCwd":            "/workspace",
		"allowlist":             []any{"git"},
		"agents":                []any{"assistant"},
	}

	params := buildWorkstationCreateParams("coding-agent", spec)

	if params["workstationKey"] != "coding-agent" {
		t.Errorf("workstationKey = %v, want coding-agent", params["workstationKey"])
	}
	if params["name"] != "Coding Agent" {
		t.Errorf("name = %v, want Coding Agent", params["name"])
	}
	if params["backendType"] != "ssh" {
		t.Errorf("backendType = %v, want ssh", params["backendType"])
	}
	if params["defaultCwd"] != "/workspace" {
		t.Errorf("defaultCwd = %v, want /workspace", params["defaultCwd"])
	}

	meta, ok := params["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata missing or wrong type")
	}
	if meta["host"] != "ssh.example.com" {
		t.Errorf("metadata.host = %v, want ssh.example.com", meta["host"])
	}
	if meta["privateKey"] != "PRIVATEKEY" {
		t.Errorf("metadata.privateKey = %v, want PRIVATEKEY", meta["privateKey"])
	}

	// allowlist and agents must not leak into RPC params (handled via side-effects).
	if _, ok := params["allowlist"]; ok {
		t.Error("allowlist should not appear in workstations.create params")
	}
	if _, ok := params["agents"]; ok {
		t.Error("agents should not appear in workstations.create params")
	}
}

func TestWorkstation_WriteOnlyFields(t *testing.T) {
	fields := manifest.WriteOnlyFields(manifest.KindWorkstation)
	required := []string{"privateKey", "allowlist", "agents", "host", "user"}
	fieldSet := make(map[string]bool, len(fields))
	for _, f := range fields {
		fieldSet[f] = true
	}
	for _, req := range required {
		if !fieldSet[req] {
			t.Errorf("expected write-only field %q to be registered for KindWorkstation", req)
		}
	}
}
