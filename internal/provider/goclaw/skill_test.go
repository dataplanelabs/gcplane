package goclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
)

func TestSkill_Observe_Found(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/skills" {
			json.NewEncoder(w).Encode(map[string]any{
				"skills": []map[string]any{
					{"id": "s1", "key": "web-search", "slug": "web-search", "enabled": true},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindSkill, "web-search")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", result["enabled"])
	}
}

func TestSkill_Observe_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"skills": []map[string]any{}})
	}))
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindSkill, "missing-skill")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestSkill_Update_FiltersToWritableFields(t *testing.T) {
	var putBody map[string]any

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills":
			json.NewEncoder(w).Encode(map[string]any{
				"skills": []map[string]any{
					{"id": "s-uuid", "slug": "web-search"},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/skills/s-uuid":
			json.NewDecoder(r.Body).Decode(&putBody)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(putBody)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents":
			// Empty tenant — applySkillGrants finds nothing to add/remove.
			json.NewEncoder(w).Encode(map[string]any{"agents": []map[string]any{}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	spec := map[string]any{
		"description": "updated desc",
		"visibility":  "tenant",
		"tags":        []any{"a", "b"},
		"status":      "active",
		"version":     7,
		// Observed-only / forbidden fields — must be filtered out:
		"enabled":     false,
		"isSystem":    true,
		"missingDeps": []any{"foo"},
		"filePath":    "/etc/whatever",
	}
	if err := p.Update(context.Background(), manifest.KindSkill, "web-search", spec); err != nil {
		t.Fatalf("update: %v", err)
	}

	mustHave := []string{"description", "visibility", "tags", "status", "version"}
	for _, k := range mustHave {
		if _, ok := putBody[k]; !ok {
			t.Errorf("expected PUT body to contain %q, got %v", k, putBody)
		}
	}
	mustNotHave := []string{"enabled", "is_system", "missing_deps", "file_path"}
	for _, k := range mustNotHave {
		if _, ok := putBody[k]; ok {
			t.Errorf("expected PUT body to OMIT %q, got %v", k, putBody)
		}
	}
}

func TestSkill_Observe_MatchesBySlug(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"skills": []map[string]any{
				{
					"id":          "s1",
					"slug":        "web-search",
					"name":        "Web Search",
					"description": "Search the web",
					"visibility":  "public",
					"version":     3,
					"tags":        []string{"net", "fetch"},
					"file_path":   "/tmp/should-be-stripped",
				},
			},
		})
	}))
	defer cleanup()

	result, err := p.Observe(context.Background(), manifest.KindSkill, "web-search")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil")
	}
	if result["slug"] != "web-search" {
		t.Errorf("missing slug, got %v", result)
	}
	if _, leaked := result["filePath"]; leaked {
		t.Errorf("file_path leaked into observed result: %v", result)
	}
	if _, leaked := result["missingDeps"]; leaked {
		t.Errorf("missing_deps leaked into observed result: %v", result)
	}
}

func TestSkill_Delete_AlreadyAbsent(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"skills": []map[string]any{}})
	}))
	defer cleanup()

	if err := p.Delete(context.Background(), manifest.KindSkill, "missing"); err != nil {
		t.Fatalf("expected idempotent delete of absent skill, got: %v", err)
	}
}

func TestSkill_Delete_CallsAPI(t *testing.T) {
	deleted := false
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills":
			json.NewEncoder(w).Encode(map[string]any{
				"skills": []map[string]any{{"id": "uuid-doom", "slug": "to-delete"}},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/skills/uuid-doom":
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	if err := p.Delete(context.Background(), manifest.KindSkill, "to-delete"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !deleted {
		t.Error("expected DELETE /v1/skills/uuid-doom")
	}
}

func TestSkill_Create_MissingSourceDir(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer cleanup()

	err := p.Create(context.Background(), manifest.KindSkill, "no-source", map[string]any{
		"description": "missing sourceDir",
	})
	if err == nil {
		t.Fatal("expected error for missing sourceDir")
	}
}

func TestSkill_Create_UploadsZIP(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: Test Skill\ndescription: hi\n---\nbody\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	gotMultipart := false
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/skills/upload" {
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "multipart/form-data") {
				t.Errorf("expected multipart Content-Type, got %q", ct)
			}
			if err := r.ParseMultipartForm(8 << 20); err != nil {
				t.Errorf("parse multipart: %v", err)
			}
			if _, _, err := r.FormFile("file"); err != nil {
				t.Errorf("expected 'file' form part: %v", err)
			} else {
				gotMultipart = true
			}
			if got := r.FormValue("source"); got != "gcplane" {
				t.Errorf("expected source=gcplane form field, got %q", got)
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"slug": "test-skill", "version": 1})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	err := p.Create(context.Background(), manifest.KindSkill, "test-skill", map[string]any{
		"sourceDir": dir,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !gotMultipart {
		t.Error("server never saw a multipart upload")
	}
}

func TestSkill_Update_WithSourceDirUploadsZIP(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: Test Skill\nslug: test-skill\ndescription: hi\n---\nbody\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	gotUpload := false
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/skills/upload":
			gotUpload = true
			if err := r.ParseMultipartForm(8 << 20); err != nil {
				t.Errorf("parse multipart: %v", err)
			}
			if got := r.FormValue("source"); got != "gcplane" {
				t.Errorf("expected source=gcplane form field, got %q", got)
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"slug": "test-skill", "status": "unchanged"})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills":
			json.NewEncoder(w).Encode(map[string]any{
				"skills": []map[string]any{{"id": "skill-uuid", "slug": "test-skill"}},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/skills/skill-uuid":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents":
			json.NewEncoder(w).Encode(map[string]any{"agents": []map[string]any{}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	if err := p.Update(context.Background(), manifest.KindSkill, "test-skill", map[string]any{
		"sourceDir": dir,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if !gotUpload {
		t.Fatal("expected update with sourceDir to upload skill package")
	}
}

// TestSkill_Update_ReconcilesGrants verifies update path: when spec declares
// grants.agents=[van-anh], a currently-granted [old-bot] is revoked and
// van-anh is granted.
func TestSkill_Update_ReconcilesGrants(t *testing.T) {
	var grantedAgentIDs []string
	var revokedAgentIDs []string

	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills":
			json.NewEncoder(w).Encode(map[string]any{
				"skills": []map[string]any{{"id": "skill-uuid", "slug": "sales-of-day"}},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/skills/skill-uuid":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents":
			json.NewEncoder(w).Encode(map[string]any{
				"agents": []map[string]any{
					{"id": "agent-van-anh", "agent_key": "van-anh"},
					{"id": "agent-old-bot", "agent_key": "old-bot"},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents/agent-van-anh/skills":
			// van-anh: not yet granted
			json.NewEncoder(w).Encode(map[string]any{
				"skills": []map[string]any{{"id": "skill-uuid", "slug": "sales-of-day", "granted": false}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents/agent-old-bot/skills":
			// old-bot: currently granted, must be revoked
			json.NewEncoder(w).Encode(map[string]any{
				"skills": []map[string]any{{"id": "skill-uuid", "slug": "sales-of-day", "granted": true}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/skills/skill-uuid/grants/agent":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			grantedAgentIDs = append(grantedAgentIDs, body["agent_id"].(string))
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodDelete:
			// /v1/skills/skill-uuid/grants/agent/{agentID}
			parts := strings.Split(r.URL.Path, "/")
			revokedAgentIDs = append(revokedAgentIDs, parts[len(parts)-1])
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	spec := map[string]any{
		"description": "daily sales summary",
		"grants":      map[string]any{"agents": []any{"van-anh"}},
	}
	if err := p.Update(context.Background(), manifest.KindSkill, "sales-of-day", spec); err != nil {
		t.Fatalf("update: %v", err)
	}

	if len(grantedAgentIDs) != 1 || grantedAgentIDs[0] != "agent-van-anh" {
		t.Errorf("expected grant for agent-van-anh, got %v", grantedAgentIDs)
	}
	if len(revokedAgentIDs) != 1 || revokedAgentIDs[0] != "agent-old-bot" {
		t.Errorf("expected revoke for agent-old-bot, got %v", revokedAgentIDs)
	}
}

func TestSkill_ListAll(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"skills": []map[string]any{
				{"slug": "skill-a", "created_by": "gcplane"},
				{"slug": "skill-b", "created_by": "system"},
			},
		})
	}))
	defer cleanup()

	infos, err := p.ListAll(context.Background(), manifest.KindSkill)
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2, got %d", len(infos))
	}
	if infos[0].Name != "skill-a" {
		t.Errorf("expected skill-a, got %s", infos[0].Name)
	}
}
