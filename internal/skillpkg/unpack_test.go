package skillpkg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnpackTo_WritesFiles(t *testing.T) {
	out := t.TempDir()
	files := []SkillFile{
		{Path: "SKILL.md", Data: []byte("---\nname: foo\n---\nbody\n")},
		{Path: "scripts/run.sh", Data: []byte("#!/bin/sh\necho hi\n")},
	}
	changed, err := UnpackTo(out, files, false)
	if err != nil {
		t.Fatalf("UnpackTo: %v", err)
	}
	if len(changed) != 2 {
		t.Fatalf("expected 2 changed files, got %d: %v", len(changed), changed)
	}
	for _, f := range files {
		got, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(f.Path)))
		if err != nil {
			t.Errorf("read %s: %v", f.Path, err)
		}
		if string(got) != string(f.Data) {
			t.Errorf("%s: content mismatch", f.Path)
		}
	}
}

func TestUnpackTo_IdempotentNoDiff(t *testing.T) {
	out := t.TempDir()
	files := []SkillFile{
		{Path: "SKILL.md", Data: []byte("---\nname: bar\n---\n")},
	}
	// First write.
	if _, err := UnpackTo(out, files, false); err != nil {
		t.Fatalf("first unpack: %v", err)
	}
	// Second write: identical content → nothing changed.
	changed, err := UnpackTo(out, files, false)
	if err != nil {
		t.Fatalf("second unpack: %v", err)
	}
	if len(changed) != 0 {
		t.Errorf("expected 0 changes on second identical unpack, got %v", changed)
	}
}

func TestUnpackTo_RejectsTraversal(t *testing.T) {
	out := t.TempDir()
	files := []SkillFile{
		{Path: "../escape.txt", Data: []byte("evil")},
	}
	_, err := UnpackTo(out, files, false)
	if err == nil {
		t.Fatal("expected error for '..' traversal")
	}
}

func TestUnpackTo_RejectsAbsolutePath(t *testing.T) {
	out := t.TempDir()
	files := []SkillFile{
		{Path: "/etc/passwd", Data: []byte("evil")},
	}
	_, err := UnpackTo(out, files, false)
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
}

func TestUnpackTo_SkipsHiddenFiles(t *testing.T) {
	out := t.TempDir()
	files := []SkillFile{
		{Path: "SKILL.md", Data: []byte("---\nname: test\n---\n")},
		{Path: ".DS_Store", Data: []byte("junk")},
	}
	changed, err := UnpackTo(out, files, false)
	if err != nil {
		t.Fatalf("UnpackTo: %v", err)
	}
	// Only SKILL.md should be written.
	if len(changed) != 1 {
		t.Errorf("expected 1 changed file, got %d: %v", len(changed), changed)
	}
	if _, err := os.Stat(filepath.Join(out, ".DS_Store")); !os.IsNotExist(err) {
		t.Error(".DS_Store should not exist in output")
	}
}

func TestUnpackTo_MaxFileSizeRejected(t *testing.T) {
	out := t.TempDir()
	big := make([]byte, MaxFileSize+1)
	files := []SkillFile{
		{Path: "big.bin", Data: big},
	}
	_, err := UnpackTo(out, files, false)
	if err == nil {
		t.Fatal("expected error for oversized file")
	}
}

func TestUnpackTo_PruneDeletesLocalOnly(t *testing.T) {
	out := t.TempDir()
	// Pre-create a local file not in server response.
	localOnly := filepath.Join(out, "local-only.txt")
	if err := os.WriteFile(localOnly, []byte("local"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	files := []SkillFile{
		{Path: "SKILL.md", Data: []byte("---\nname: x\n---\n")},
	}

	// prune=false: local-only file preserved.
	if _, err := UnpackTo(out, files, false); err != nil {
		t.Fatalf("unpack (no prune): %v", err)
	}
	if _, err := os.Stat(localOnly); err != nil {
		t.Error("local-only file should be preserved when prune=false")
	}

	// prune=true: local-only file deleted.
	if _, err := UnpackTo(out, files, true); err != nil {
		t.Fatalf("unpack (prune): %v", err)
	}
	if _, err := os.Stat(localOnly); !os.IsNotExist(err) {
		t.Error("local-only file should be deleted when prune=true")
	}
}

func TestUnpackTo_EmptyOutDir(t *testing.T) {
	_, err := UnpackTo("", []SkillFile{{Path: "x.txt", Data: []byte("x")}}, false)
	if err == nil {
		t.Fatal("expected error for empty outDir")
	}
}
