package goclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

// agentsHandlerForLinks returns an HTTP handler that serves /v1/agents with the
// given agent fixtures keyed by agent_key → id.
func agentsHandlerForLinks(t *testing.T, agents []map[string]any) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/agents" {
			_ = json.NewEncoder(w).Encode(map[string]any{"agents": agents})
			return
		}
		http.NotFound(w, r)
	}
}

func TestAgentLink_ParseName(t *testing.T) {
	cases := []struct {
		in       string
		src, tgt string
		ok       bool
	}{
		{"planner--coder", "planner", "coder", true},
		{"a--b", "a", "b", true},
		{"missing-target", "", "", false},
		{"--orphan", "", "", false},
		{"orphan--", "", "", false},
		{"", "", "", false},
		// 3+ segments rejected — agent_keys may not contain "--"
		{"a--b--c", "", "", false},
		{"a--b--c--d", "", "", false},
	}
	for _, c := range cases {
		s, tg, err := parseAgentLinkName(c.in)
		if c.ok && err != nil {
			t.Errorf("parseAgentLinkName(%q): unexpected error %v", c.in, err)
		}
		if !c.ok && err == nil {
			t.Errorf("parseAgentLinkName(%q): expected error", c.in)
		}
		if c.ok && (s != c.src || tg != c.tgt) {
			t.Errorf("parseAgentLinkName(%q): got (%q,%q), want (%q,%q)", c.in, s, tg, c.src, c.tgt)
		}
	}
}

func TestAgentLink_Observe_Found(t *testing.T) {
	agents := []map[string]any{
		{"id": "src-id", "agent_key": "planner"},
		{"id": "tgt-id", "agent_key": "coder"},
	}
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "agents.links.list", ok: true, payload: map[string]any{
			"links": []map[string]any{
				{
					"id":              "link-uuid",
					"source_agent_id": "src-id",
					"target_agent_id": "tgt-id",
					"direction":       "outbound",
					"description":     "Planner delegates to Coder",
					"max_concurrent":  3,
					"status":          "active",
					"created_by":      "gcplane",
				},
			},
		}},
	}, agentsHandlerForLinks(t, agents))
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindAgentLink, "planner--coder")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["direction"] != "outbound" {
		t.Errorf("direction: got %v, want outbound", result["direction"])
	}
	if result["description"] != "Planner delegates to Coder" {
		t.Errorf("description mismatch: %v", result["description"])
	}
	if result["status"] != "active" {
		t.Errorf("status: got %v, want active", result["status"])
	}
	// id must remain (needed for update/delete)
	if result["id"] != "link-uuid" {
		t.Errorf("id stripped or mismatched: %v", result["id"])
	}
	// Joined / identity fields must be stripped
	for _, f := range []string{"sourceAgentId", "targetAgentId", "sourceAgentKey", "targetAgentKey", "teamId"} {
		if _, has := result[f]; has {
			t.Errorf("internal field %q should be stripped", f)
		}
	}
}

func TestAgentLink_Observe_NotFound_NoMatch(t *testing.T) {
	agents := []map[string]any{
		{"id": "src-id", "agent_key": "planner"},
		{"id": "tgt-id", "agent_key": "coder"},
		{"id": "other-id", "agent_key": "other"},
	}
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "agents.links.list", ok: true, payload: map[string]any{
			"links": []map[string]any{
				// link from planner exists but to a different target
				{
					"id":              "link-uuid",
					"source_agent_id": "src-id",
					"target_agent_id": "other-id",
					"direction":       "outbound",
					"status":          "active",
				},
			},
		}},
	}, agentsHandlerForLinks(t, agents))
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindAgentLink, "planner--coder")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestAgentLink_Observe_BadName(t *testing.T) {
	p, cleanup := newWSTestServer(t, nil, nil)
	defer cleanup()

	_, err := p.Observe(context.Background(), manifest.KindAgentLink, "missing-target")
	if err == nil {
		t.Fatal("expected error for malformed name")
	}
}

func TestAgentLink_Create(t *testing.T) {
	agents := []map[string]any{
		{"id": "src-id", "agent_key": "planner"},
		{"id": "tgt-id", "agent_key": "coder"},
	}
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "agents.links.create", ok: true, payload: map[string]any{"link": map[string]any{"id": "new-link"}}},
	}, agentsHandlerForLinks(t, agents))
	defer cleanup()

	err := p.Create(context.Background(), manifest.KindAgentLink, "planner--coder", map[string]any{
		"direction":     "outbound",
		"description":   "delegates impl",
		"maxConcurrent": 5,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
}

func TestAgentLink_Update(t *testing.T) {
	agents := []map[string]any{
		{"id": "src-id", "agent_key": "planner"},
		{"id": "tgt-id", "agent_key": "coder"},
	}
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "agents.links.list", ok: true, payload: map[string]any{
			"links": []map[string]any{
				{
					"id":              "link-uuid",
					"source_agent_id": "src-id",
					"target_agent_id": "tgt-id",
					"direction":       "outbound",
					"status":          "active",
				},
			},
		}},
		{method: "agents.links.update", ok: true, payload: map[string]any{"ok": true}},
	}, agentsHandlerForLinks(t, agents))
	defer cleanup()

	err := p.Update(context.Background(), manifest.KindAgentLink, "planner--coder", map[string]any{
		"description":   "updated",
		"maxConcurrent": 10,
		"status":        "disabled",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
}

func TestAgentLink_Update_NotFound(t *testing.T) {
	agents := []map[string]any{
		{"id": "src-id", "agent_key": "planner"},
		{"id": "tgt-id", "agent_key": "coder"},
	}
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "agents.links.list", ok: true, payload: map[string]any{"links": []map[string]any{}}},
	}, agentsHandlerForLinks(t, agents))
	defer cleanup()

	err := p.Update(context.Background(), manifest.KindAgentLink, "planner--coder", map[string]any{"description": "x"})
	if err == nil {
		t.Fatal("expected error updating non-existent link")
	}
}

func TestAgentLink_Delete(t *testing.T) {
	agents := []map[string]any{
		{"id": "src-id", "agent_key": "planner"},
		{"id": "tgt-id", "agent_key": "coder"},
	}
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "agents.links.list", ok: true, payload: map[string]any{
			"links": []map[string]any{
				{
					"id":              "link-uuid",
					"source_agent_id": "src-id",
					"target_agent_id": "tgt-id",
					"direction":       "outbound",
					"status":          "active",
				},
			},
		}},
		{method: "agents.links.delete", ok: true, payload: map[string]any{"ok": true}},
	}, agentsHandlerForLinks(t, agents))
	defer cleanup()

	if err := p.Delete(context.Background(), manifest.KindAgentLink, "planner--coder"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestAgentLink_Delete_Idempotent(t *testing.T) {
	agents := []map[string]any{
		{"id": "src-id", "agent_key": "planner"},
		{"id": "tgt-id", "agent_key": "coder"},
	}
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "agents.links.list", ok: true, payload: map[string]any{"links": []map[string]any{}}},
	}, agentsHandlerForLinks(t, agents))
	defer cleanup()

	if err := p.Delete(context.Background(), manifest.KindAgentLink, "planner--coder"); err != nil {
		t.Fatalf("idempotent delete should not error: %v", err)
	}
}

func TestAgentLink_ListAll_SkipsTeamManaged(t *testing.T) {
	// One source agent (planner), one target (coder, no outbound links).
	// Mock returns the canned response for every agents.links.list call,
	// but we only expect emissions for planner → coder. Since coder is also
	// queried as a source, we use a smart handler that returns empty for coder.
	agents := []map[string]any{
		{"id": "src-id", "agent_key": "planner"},
		{"id": "tgt-id", "agent_key": "coder"},
	}
	p, cleanup := newWSTestServerSmart(t, agents, map[string][]map[string]any{
		"src-id": {
			{
				"id":              "user-link",
				"source_agent_id": "src-id",
				"target_agent_id": "tgt-id",
				"direction":       "outbound",
				"status":          "active",
				"created_by":      "gcplane",
			},
			{
				"id":              "team-link",
				"source_agent_id": "src-id",
				"target_agent_id": "tgt-id",
				"team_id":         "team-uuid", // managed by AgentTeam — must be skipped
				"direction":       "outbound",
				"status":          "active",
				"created_by":      "gcplane",
			},
		},
		"tgt-id": {}, // coder has no outbound links
	})
	defer cleanup()

	infos, err := p.ListAll(context.Background(), manifest.KindAgentLink)
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 user-link (team-link skipped), got %d: %+v", len(infos), infos)
	}
	got := infos[0]
	if got.Kind != manifest.KindAgentLink {
		t.Errorf("kind: got %s, want AgentLink", got.Kind)
	}
	if got.Name != "planner--coder" {
		t.Errorf("name: got %s, want planner--coder", got.Name)
	}
	if got.CreatedBy != "gcplane" {
		t.Errorf("created_by: got %s, want gcplane", got.CreatedBy)
	}
}
