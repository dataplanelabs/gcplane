package goclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

func TestCronJob_Observe_Found(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "cron.list", ok: true, payload: map[string]any{
			"jobs": []map[string]any{
				{"id": "j1", "name": "daily-sync", "schedule": "0 0 * * *"},
			},
		}},
	}, nil)
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindCronJob, "daily-sync")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["name"] != "daily-sync" {
		t.Errorf("expected name=daily-sync, got %v", result["name"])
	}
}

func TestCronJob_Observe_WithNewFields(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "cron.list", ok: true, payload: map[string]any{
			"jobs": []map[string]any{
				{
					"id": "j1", "name": "daily-report",
					"enabled":        true,
					"stateless":      false,
					"deliver":        true,
					"deliverChannel": "telegram",
					"deliverTo":      "@admin",
					"wakeHeartbeat":  true,
					"schedule":       map[string]any{"kind": "cron", "expr": "0 9 * * *"},
				},
			},
		}},
	}, nil)
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindCronJob, "daily-report")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// New fields should be present in observe result (camelCase from API)
	if result["stateless"] != false {
		t.Errorf("expected stateless=false, got %v", result["stateless"])
	}
	if result["deliver"] != true {
		t.Errorf("expected deliver=true, got %v", result["deliver"])
	}
	if result["deliverChannel"] != "telegram" {
		t.Errorf("expected deliverChannel=telegram, got %v", result["deliverChannel"])
	}
	if result["deliverTo"] != "@admin" {
		t.Errorf("expected deliverTo=@admin, got %v", result["deliverTo"])
	}
	if result["wakeHeartbeat"] != true {
		t.Errorf("expected wakeHeartbeat=true, got %v", result["wakeHeartbeat"])
	}
}

func TestCronJob_Observe_NotFound(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "cron.list", ok: true, payload: map[string]any{"jobs": []map[string]any{}}},
	}, nil)
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindCronJob, "ghost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestCronJob_Create(t *testing.T) {
	agentsHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/agents" {
			json.NewEncoder(w).Encode(map[string]any{
				"agents": []map[string]any{
					{"id": "agent-uuid-1", "agent_key": "my-bot"},
				},
			})
			return
		}
		http.NotFound(w, r)
	}

	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "cron.create", ok: true, payload: map[string]any{"ok": true}},
	}, agentsHandler)
	defer cleanup()

	err := p.Create(context.Background(), manifest.KindCronJob, "daily-sync", map[string]any{
		"schedule": "0 0 * * *",
		"agentKey": "my-bot",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
}

func TestCronJob_Create_WithNewFields(t *testing.T) {
	agentsHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/agents" {
			json.NewEncoder(w).Encode(map[string]any{
				"agents": []map[string]any{
					{"id": "agent-uuid-1", "agent_key": "report-bot"},
				},
			})
			return
		}
		http.NotFound(w, r)
	}

	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "cron.create", ok: true, payload: map[string]any{"ok": true}},
	}, agentsHandler)
	defer cleanup()

	err := p.Create(context.Background(), manifest.KindCronJob, "daily-report", map[string]any{
		"schedule": map[string]any{
			"kind": "cron",
			"expr": "0 9 * * *",
			"tz":   "Asia/Saigon",
		},
		"message":        "Generate daily report",
		"agentKey":       "report-bot",
		"enabled":        true,
		"deliver":        true,
		"deliverChannel": "telegram",
		"deliverTo":      "@admin",
		"wakeHeartbeat":  true,
		"stateless":      false,
	})
	if err != nil {
		t.Fatalf("create with new fields: %v", err)
	}
}

func TestCronJob_Update(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "cron.list", ok: true, payload: map[string]any{
			"jobs": []map[string]any{
				{"id": "j-uuid", "name": "daily-sync"},
			},
		}},
		{method: "cron.update", ok: true, payload: map[string]any{"ok": true}},
	}, nil)
	defer cleanup()

	err := p.Update(context.Background(), manifest.KindCronJob, "daily-sync", map[string]any{"schedule": "0 6 * * *"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
}

func TestCronJob_Update_WithNewFields(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "cron.list", ok: true, payload: map[string]any{
			"jobs": []map[string]any{
				{"id": "j-uuid", "name": "daily-report"},
			},
		}},
		{method: "cron.update", ok: true, payload: map[string]any{"ok": true}},
	}, nil)
	defer cleanup()

	err := p.Update(context.Background(), manifest.KindCronJob, "daily-report", map[string]any{
		"deliver":        true,
		"deliverChannel": "slack",
		"deliverTo":      "#reports",
		"stateless":      true,
	})
	if err != nil {
		t.Fatalf("update with new fields: %v", err)
	}
}

func TestCronJob_Update_NotFound(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "cron.list", ok: true, payload: map[string]any{"jobs": []map[string]any{}}},
	}, nil)
	defer cleanup()

	err := p.Update(context.Background(), manifest.KindCronJob, "ghost", map[string]any{})
	if err == nil {
		t.Fatal("expected error updating non-existent cron job")
	}
}

func TestCronJob_Delete(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "cron.list", ok: true, payload: map[string]any{
			"jobs": []map[string]any{
				{"id": "j-uuid", "name": "daily-sync"},
			},
		}},
		{method: "cron.delete", ok: true, payload: map[string]any{"ok": true}},
	}, nil)
	defer cleanup()

	if err := p.Delete(context.Background(), manifest.KindCronJob, "daily-sync"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestCronJob_Delete_NotFound(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "cron.list", ok: true, payload: map[string]any{"jobs": []map[string]any{}}},
	}, nil)
	defer cleanup()

	if err := p.Delete(context.Background(), manifest.KindCronJob, "ghost"); err != nil {
		t.Fatalf("idempotent delete should not error: %v", err)
	}
}

func TestCronJob_ListAll(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "cron.list", ok: true, payload: map[string]any{
			"jobs": []map[string]any{
				{"name": "job-a", "created_by": "gcplane"},
				{"name": "job-b", "created_by": "ui"},
			},
		}},
	}, nil)
	defer cleanup()

	infos, err := p.ListAll(context.Background(), manifest.KindCronJob)
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2, got %d", len(infos))
	}
	if infos[0].Name != "job-a" {
		t.Errorf("expected job-a, got %s", infos[0].Name)
	}
	if infos[1].CreatedBy != "ui" {
		t.Errorf("expected ui, got %s", infos[1].CreatedBy)
	}
}
