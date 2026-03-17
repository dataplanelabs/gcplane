package goclaw

import (
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

	result, err := p.Observe(manifest.KindCronJob, "daily-sync")
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

func TestCronJob_Observe_NotFound(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "cron.list", ok: true, payload: map[string]any{"jobs": []map[string]any{}}},
	}, nil)
	defer cleanup()

	result, err := p.Observe(manifest.KindCronJob, "ghost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestCronJob_Create(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "cron.create", ok: true, payload: map[string]any{"ok": true}},
	}, nil)
	defer cleanup()

	err := p.Create(manifest.KindCronJob, "daily-sync", map[string]any{
		"schedule": "0 0 * * *",
		"agentKey": "my-bot",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
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

	err := p.Update(manifest.KindCronJob, "daily-sync", map[string]any{"schedule": "0 6 * * *"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
}

func TestCronJob_Update_NotFound(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "cron.list", ok: true, payload: map[string]any{"jobs": []map[string]any{}}},
	}, nil)
	defer cleanup()

	err := p.Update(manifest.KindCronJob, "ghost", map[string]any{})
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

	if err := p.Delete(manifest.KindCronJob, "daily-sync"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestCronJob_Delete_NotFound(t *testing.T) {
	p, cleanup := newWSTestServer(t, []wsResponse{
		{method: "cron.list", ok: true, payload: map[string]any{"jobs": []map[string]any{}}},
	}, nil)
	defer cleanup()

	if err := p.Delete(manifest.KindCronJob, "ghost"); err != nil {
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

	infos, err := p.ListAll(manifest.KindCronJob)
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
