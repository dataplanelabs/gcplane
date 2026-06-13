package goclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestDownloadSkill_HappyPath(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills":
			json.NewEncoder(w).Encode(map[string]any{
				"skills": []map[string]any{
					{"id": "skill-123", "slug": "my-skill"},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills/skill-123/files":
			json.NewEncoder(w).Encode(map[string]any{
				"files": []map[string]any{
					{"path": "SKILL.md", "name": "SKILL.md", "isDir": false, "size": 20},
					{"path": "scripts/run.sh", "name": "run.sh", "isDir": false, "size": 10},
					{"path": "scripts", "name": "scripts", "isDir": true},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills/skill-123/files/SKILL.md":
			w.Write([]byte("---\nname: My Skill\n---\nbody\n"))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills/skill-123/files/scripts/run.sh":
			w.Write([]byte("#!/bin/sh\necho hi\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	files, err := p.DownloadSkill(context.Background(), "my-skill")
	if err != nil {
		t.Fatalf("DownloadSkill: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	byPath := make(map[string][]byte)
	for _, f := range files {
		byPath[f.Path] = f.Data
	}
	if string(byPath["SKILL.md"]) != "---\nname: My Skill\n---\nbody\n" {
		t.Errorf("SKILL.md content mismatch: %q", byPath["SKILL.md"])
	}
	if string(byPath["scripts/run.sh"]) != "#!/bin/sh\necho hi\n" {
		t.Errorf("run.sh content mismatch: %q", byPath["scripts/run.sh"])
	}
}

func TestDownloadSkill_NotFound(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"skills": []map[string]any{}})
	}))
	defer cleanup()

	_, err := p.DownloadSkill(context.Background(), "missing-skill")
	if err == nil {
		t.Fatal("expected error for missing skill")
	}
}

func TestDownloadSkill_EmptyFileTree(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills":
			json.NewEncoder(w).Encode(map[string]any{
				"skills": []map[string]any{{"id": "sid", "slug": "empty-skill"}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills/sid/files":
			json.NewEncoder(w).Encode(map[string]any{"files": []map[string]any{}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	files, err := p.DownloadSkill(context.Background(), "empty-skill")
	if err != nil {
		t.Fatalf("DownloadSkill: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files for empty tree, got %d", len(files))
	}
}

func TestDownloadSkillSource_IncludesGrants(t *testing.T) {
	p, cleanup := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills":
			json.NewEncoder(w).Encode(map[string]any{
				"skills": []map[string]any{{"id": "sk1", "slug": "granted-skill"}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills/sk1/files":
			json.NewEncoder(w).Encode(map[string]any{
				"files": []map[string]any{
					{"path": "SKILL.md", "name": "SKILL.md", "isDir": false},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills/sk1/files/SKILL.md":
			w.Write([]byte("---\nname: Granted Skill\n---\n"))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents":
			json.NewEncoder(w).Encode(map[string]any{
				"agents": []map[string]any{
					{"id": "a1", "agent_key": "bot-alpha"},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents/a1/skills":
			json.NewEncoder(w).Encode(map[string]any{
				"skills": []map[string]any{
					{"id": "sk1", "slug": "granted-skill", "granted": true},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	files, grantees, err := p.DownloadSkillSource(context.Background(), "granted-skill")
	if err != nil {
		t.Fatalf("DownloadSkillSource: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
	if len(grantees) != 1 || grantees[0] != "bot-alpha" {
		t.Errorf("expected grantees=[bot-alpha], got %v", grantees)
	}
}
