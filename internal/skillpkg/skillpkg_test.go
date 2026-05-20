package skillpkg

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestPackDir_BasicSkill(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "SKILL.md"), "---\nname: foo\n---\nbody\n")
	writeFile(t, filepath.Join(dir, "scripts", "run.sh"), "#!/bin/sh\necho hi\n")
	writeFile(t, filepath.Join(dir, ".DS_Store"), "junk") // skipped
	writeFile(t, filepath.Join(dir, ".git", "HEAD"), "junk") // skipped

	res, err := PackDir(dir)
	if err != nil {
		t.Fatalf("PackDir: %v", err)
	}
	if res.SkillMD == "" {
		t.Error("SkillMD empty")
	}

	zr, err := zip.NewReader(bytes.NewReader(res.ZIP), int64(len(res.ZIP)))
	if err != nil {
		t.Fatalf("zip read: %v", err)
	}
	names := make(map[string]bool, len(zr.File))
	for _, f := range zr.File {
		names[f.Name] = true
	}
	want := []string{"SKILL.md", "scripts/run.sh"}
	for _, n := range want {
		if !names[n] {
			t.Errorf("missing %q in zip; got %v", n, names)
		}
	}
	if names[".DS_Store"] || names[".git/HEAD"] {
		t.Errorf("hidden files leaked into zip: %v", names)
	}
}

func TestPackDir_Deterministic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "SKILL.md"), "---\nname: foo\n---\n")
	writeFile(t, filepath.Join(dir, "b.txt"), "b")
	writeFile(t, filepath.Join(dir, "a.txt"), "a")

	r1, err := PackDir(dir)
	if err != nil {
		t.Fatalf("first pack: %v", err)
	}
	r2, err := PackDir(dir)
	if err != nil {
		t.Fatalf("second pack: %v", err)
	}
	if !bytes.Equal(r1.ZIP, r2.ZIP) {
		t.Error("zip bytes differ between runs — packer is not deterministic")
	}
	if r1.ContentHash != r2.ContentHash {
		t.Errorf("hash differs: %s vs %s", r1.ContentHash, r2.ContentHash)
	}
}

func TestPackDir_MissingSkillMD(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "other.txt"), "x")
	if _, err := PackDir(dir); err == nil {
		t.Fatal("expected error for missing SKILL.md")
	}
}
