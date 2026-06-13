package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
)

func skillInfo(source string, system bool) reconciler.ResourceInfo {
	return reconciler.ResourceInfo{
		Kind:     manifest.KindSkill,
		Name:     "demo-skill",
		Source:   source,
		IsSystem: system,
	}
}

func TestShouldPullSkill_DefaultSourceFilter(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		system     bool
		localKnown bool
		want       bool
	}{
		{"evolution known", "evolution", false, true, true},
		{"gcplane known", "gcplane", false, true, true},
		{"evolution unknown locally", "evolution", false, false, false},
		{"gcplane unknown locally", "gcplane", false, false, false},
		{"cli source skipped", "cli", false, true, false},
		{"unknown source skipped", "unknown", false, true, false},
		{"bundled source skipped", "bundled", false, true, false},
		{"empty source skipped", "", false, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldPullSkill(skillInfo(tt.source, tt.system), false, tt.localKnown)
			if got != tt.want {
				t.Fatalf("shouldPullSkill(source=%q, all=false, local=%v) = %v, want %v",
					tt.source, tt.localKnown, got, tt.want)
			}
		})
	}
}

func TestShouldPullSkill_AllMode(t *testing.T) {
	tests := []struct {
		name   string
		source string
		system bool
		want   bool
	}{
		{"evolution pulled", "evolution", false, true},
		{"gcplane pulled", "gcplane", false, true},
		{"cli pulled under all", "cli", false, true},
		{"unknown pulled under all", "unknown", false, true},
		{"bundled excluded", "bundled", false, false},
		{"is_system excluded", "evolution", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// localKnown is irrelevant under --all.
			got := shouldPullSkill(skillInfo(tt.source, tt.system), true, false)
			if got != tt.want {
				t.Fatalf("shouldPullSkill(source=%q, system=%v, all=true) = %v, want %v",
					tt.source, tt.system, got, tt.want)
			}
		})
	}
}

func TestWriteFrontmatter_WritesGrants(t *testing.T) {
	dir := t.TempDir()
	if err := writeFrontmatter(dir, []string{"van-anh", "annhien"}); err != nil {
		t.Fatalf("writeFrontmatter: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "frontmatter.yaml"))
	if err != nil {
		t.Fatalf("read frontmatter: %v", err)
	}
	got := string(data)
	// Sorted grantees.
	if want := "grants:\n    agents:\n        - annhien\n        - van-anh\n"; got != want {
		t.Fatalf("frontmatter content = %q, want %q", got, want)
	}
}

func TestWriteFrontmatter_RemovesStaleWhenGrantsRevoked(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "frontmatter.yaml")

	if err := writeFrontmatter(dir, []string{"van-anh"}); err != nil {
		t.Fatalf("seed frontmatter: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("frontmatter should exist after seed: %v", err)
	}

	// Grants revoked server-side → stale file must be removed.
	if err := writeFrontmatter(dir, nil); err != nil {
		t.Fatalf("writeFrontmatter(empty): %v", err)
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Fatalf("stale frontmatter still present, stat err = %v", err)
	}
}

func TestWriteFrontmatter_NoGrantsNoFileIsNoop(t *testing.T) {
	dir := t.TempDir()
	if err := writeFrontmatter(dir, nil); err != nil {
		t.Fatalf("writeFrontmatter(empty) on clean dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "frontmatter.yaml")); !os.IsNotExist(err) {
		t.Fatalf("no frontmatter should be created, stat err = %v", err)
	}
}
