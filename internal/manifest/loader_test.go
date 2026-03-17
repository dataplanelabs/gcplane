package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_SingleFile(t *testing.T) {
	dir := t.TempDir()
	content := `apiVersion: gcplane.io/v1
kind: Manifest
resources:
  - kind: Provider
    name: anthropic
    spec:
      type: anthropic
`
	path := filepath.Join(dir, "manifest.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	m, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Resources) != 1 {
		t.Errorf("expected 1 resource, got %d", len(m.Resources))
	}
}

func TestLoad_Directory_MergesResources(t *testing.T) {
	dir := t.TempDir()

	file1 := `apiVersion: gcplane.io/v1
kind: Manifest
connection:
  endpoint: http://localhost:8080
  token: tok
metadata:
  name: prod
resources:
  - kind: Provider
    name: anthropic
    spec:
      type: anthropic
`
	file2 := `apiVersion: gcplane.io/v1
kind: Manifest
resources:
  - kind: Agent
    name: bot
    spec:
      model: claude-3
`
	if err := os.WriteFile(filepath.Join(dir, "a.yaml"), []byte(file1), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.yml"), []byte(file2), 0600); err != nil {
		t.Fatal(err)
	}
	// Non-YAML file should be ignored
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignored"), 0600); err != nil {
		t.Fatal(err)
	}

	m, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(m.Resources) != 2 {
		t.Errorf("expected 2 resources, got %d", len(m.Resources))
	}
	if m.Connection.Endpoint != "http://localhost:8080" {
		t.Errorf("expected connection from first file, got %q", m.Connection.Endpoint)
	}
	if m.Metadata.Name != "prod" {
		t.Errorf("expected metadata from first file, got %q", m.Metadata.Name)
	}
}

func TestLoad_Directory_DuplicateResource_ReturnsError(t *testing.T) {
	dir := t.TempDir()

	dup := `apiVersion: gcplane.io/v1
kind: Manifest
resources:
  - kind: Provider
    name: anthropic
    spec:
      type: anthropic
`
	if err := os.WriteFile(filepath.Join(dir, "a.yaml"), []byte(dup), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.yaml"), []byte(dup), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for duplicate resource")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected 'duplicate' in error, got: %v", err)
	}
}

func TestLoad_Directory_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	m, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Resources) != 0 {
		t.Errorf("expected 0 resources for empty dir, got %d", len(m.Resources))
	}
}

func TestLoad_NonExistentPath(t *testing.T) {
	_, err := Load("/nonexistent/path/manifest.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}
