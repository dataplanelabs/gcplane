package manifest

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, dir, slug, frontmatter string) {
	t.Helper()
	skillDir := filepath.Join(dir, "skills", slug)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(frontmatter), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
}

func TestLoadSkillResources_HappyPath(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "say-hello", "---\nname: Say Hello\ndescription: greets the user\n---\nbody\n")

	overrides := []byte("overrides:\n  global-foo:\n    enabled: false\n")
	if err := os.WriteFile(filepath.Join(dir, "skill-overrides.yaml"), overrides, 0o644); err != nil {
		t.Fatalf("write overrides: %v", err)
	}

	res, err := LoadSkillResources(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 resources (1 skill + 1 override), got %d", len(res))
	}

	sort.Slice(res, func(i, j int) bool { return string(res[i].Kind) < string(res[j].Kind) })
	if res[0].Kind != KindSkill || res[0].Name != "say-hello" {
		t.Errorf("expected Skill say-hello first, got %s/%s", res[0].Kind, res[0].Name)
	}
	if res[0].Spec["visibility"] != "internal" {
		t.Errorf("expected visibility=internal for non-system dir, got %v", res[0].Spec["visibility"])
	}
	if res[0].Spec["description"] != "greets the user" {
		t.Errorf("description not parsed: %v", res[0].Spec["description"])
	}
	if _, ok := res[0].Spec["sourceDir"].(string); !ok {
		t.Errorf("expected sourceDir to be set, got %v", res[0].Spec["sourceDir"])
	}
	if res[1].Kind != KindSkillConfig || res[1].Name != "global-foo" {
		t.Errorf("expected SkillConfig global-foo, got %s/%s", res[1].Kind, res[1].Name)
	}
	if res[1].Spec["enabled"] != false {
		t.Errorf("expected enabled=false in override, got %v", res[1].Spec["enabled"])
	}
}

func TestLoadSkillResources_SystemDirYieldsPublicVisibility(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "_system")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeSkill(t, dir, "global-greet", "---\nname: Global Greet\n---\n")

	res, err := LoadSkillResources(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(res))
	}
	if res[0].Spec["visibility"] != "public" {
		t.Errorf("expected visibility=public for _system, got %v", res[0].Spec["visibility"])
	}
}

func TestLoadSkillResources_EmptyTreeNoError(t *testing.T) {
	dir := t.TempDir() // no skills/ subdir at all
	res, err := LoadSkillResources(dir)
	if err != nil {
		t.Fatalf("expected no error for empty tree, got: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("expected empty result, got %d", len(res))
	}
}

func TestLoadSkillResources_MissingNameField(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "bad", "---\ndescription: no name\n---\n")

	_, err := LoadSkillResources(dir)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("expected error to mention 'name', got: %v", err)
	}
}

func TestLoadSkillResources_FrontmatterOverlay(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "with-tags", "---\nname: With Tags\n---\n")

	overlay := []byte("tags:\n  - alpha\n  - beta\nstatus: active\n")
	if err := os.WriteFile(filepath.Join(dir, "skills", "with-tags", "frontmatter.yaml"), overlay, 0o644); err != nil {
		t.Fatalf("write overlay: %v", err)
	}

	res, err := LoadSkillResources(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if res[0].Spec["status"] != "active" {
		t.Errorf("status overlay not applied: %v", res[0].Spec["status"])
	}
	tags, _ := res[0].Spec["tags"].([]any)
	if len(tags) != 2 || tags[0] != "alpha" {
		t.Errorf("tags overlay not applied: %v", res[0].Spec["tags"])
	}
}

func TestLoadSkillResources_GrantsOverlay(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "with-grants", "---\nname: With Grants\n---\n")

	overlay := []byte("grants:\n  agents:\n    - van-anh\n    - assistant\n")
	if err := os.WriteFile(filepath.Join(dir, "skills", "with-grants", "frontmatter.yaml"), overlay, 0o644); err != nil {
		t.Fatalf("write overlay: %v", err)
	}

	res, err := LoadSkillResources(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	grants, ok := res[0].Spec["grants"].(map[string]any)
	if !ok {
		t.Fatalf("expected grants map, got %v", res[0].Spec["grants"])
	}
	agents, _ := grants["agents"].([]any)
	if len(agents) != 2 || agents[0] != "van-anh" || agents[1] != "assistant" {
		t.Errorf("grants.agents overlay not applied: %v", agents)
	}
}

func TestLoadDir_YamlAndFilesystemConflict(t *testing.T) {
	root := t.TempDir()
	// YAML resource
	yaml := []byte("apiVersion: gcplane.io/v1\nkind: Manifest\nmetadata:\n  name: t\nresources:\n  - kind: Skill\n    name: clash\n    spec: {}\n")
	if err := os.WriteFile(filepath.Join(root, "manifest.yaml"), yaml, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Filesystem skill with same slug
	writeSkill(t, root, "clash", "---\nname: Clash\n---\n")

	_, err := loadDir(root)
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !strings.Contains(err.Error(), "filesystem skill") {
		t.Errorf("expected 'filesystem skill' in error, got: %v", err)
	}
}

func TestParseFrontmatter_CRLF(t *testing.T) {
	content := []byte("---\r\nname: hi\r\n---\r\nbody\r\n")
	fm, err := parseFrontmatter(content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if fm.Name != "hi" {
		t.Errorf("expected name=hi, got %q", fm.Name)
	}
}

func TestParseFrontmatter_BOM(t *testing.T) {
	content := append([]byte{0xEF, 0xBB, 0xBF}, []byte("---\nname: hi\n---\n")...)
	fm, err := parseFrontmatter(content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if fm.Name != "hi" {
		t.Errorf("expected name=hi, got %q", fm.Name)
	}
}
