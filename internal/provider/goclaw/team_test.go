package goclaw

import (
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

func TestTeam_Observe_Found(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "teams.list", ok: true, payload: map[string]any{
			"teams": []map[string]any{
				{"id": "t1", "name": "alpha-team", "description": "Alpha"},
			},
		}},
	}, nil)
	defer cleanup()

	result, err := p.Observe(manifest.KindAgentTeam, "alpha-team")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["name"] != "alpha-team" {
		t.Errorf("expected name=alpha-team, got %v", result["name"])
	}
}

func TestTeam_Observe_NotFound(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "teams.list", ok: true, payload: map[string]any{"teams": []map[string]any{}}},
	}, nil)
	defer cleanup()

	result, err := p.Observe(manifest.KindAgentTeam, "ghost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestTeam_Create(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "teams.create", ok: true, payload: map[string]any{"ok": true}},
	}, nil)
	defer cleanup()

	err := p.Create(manifest.KindAgentTeam, "alpha-team", map[string]any{
		"description": "Alpha team",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
}

func TestTeam_Update(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "teams.list", ok: true, payload: map[string]any{
			"teams": []map[string]any{
				{"id": "t-uuid", "name": "alpha-team"},
			},
		}},
		{method: "teams.update", ok: true, payload: map[string]any{"ok": true}},
	}, nil)
	defer cleanup()

	err := p.Update(manifest.KindAgentTeam, "alpha-team", map[string]any{"description": "Updated"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
}

func TestTeam_Update_NotFound(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "teams.list", ok: true, payload: map[string]any{"teams": []map[string]any{}}},
	}, nil)
	defer cleanup()

	err := p.Update(manifest.KindAgentTeam, "ghost", map[string]any{})
	if err == nil {
		t.Fatal("expected error updating non-existent team")
	}
}

func TestTeam_Delete(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "teams.list", ok: true, payload: map[string]any{
			"teams": []map[string]any{
				{"id": "t-uuid", "name": "alpha-team"},
			},
		}},
		{method: "teams.delete", ok: true, payload: map[string]any{"ok": true}},
	}, nil)
	defer cleanup()

	if err := p.Delete(manifest.KindAgentTeam, "alpha-team"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestTeam_Delete_NotFound(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "teams.list", ok: true, payload: map[string]any{"teams": []map[string]any{}}},
	}, nil)
	defer cleanup()

	if err := p.Delete(manifest.KindAgentTeam, "ghost"); err != nil {
		t.Fatalf("idempotent delete should not error: %v", err)
	}
}

func TestTeam_ListAll(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "teams.list", ok: true, payload: map[string]any{
			"teams": []map[string]any{
				{"name": "team-a", "created_by": "gcplane"},
				{"name": "team-b", "created_by": "ui"},
			},
		}},
	}, nil)
	defer cleanup()

	infos, err := p.ListAll(manifest.KindAgentTeam)
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2, got %d", len(infos))
	}
	if infos[0].Name != "team-a" {
		t.Errorf("expected team-a, got %s", infos[0].Name)
	}
}
