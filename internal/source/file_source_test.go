package source

import (
	"os"
	"path/filepath"
	"testing"
)

const validManifestYAML = `apiVersion: gcplane.io/v1
kind: Manifest
metadata:
  name: test-manifest
connection:
  endpoint: http://localhost:8080
  token: secret
resources:
  - kind: Provider
    name: anthropic
    spec:
      type: anthropic
`

func TestFileSource_Fetch_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	if err := os.WriteFile(path, []byte(validManifestYAML), 0600); err != nil {
		t.Fatal(err)
	}

	fs := NewFileSource(path)
	m, hash, err := fs.Fetch()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if m == nil {
		t.Fatal("expected manifest, got nil")
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
	if len(hash) != 64 {
		t.Errorf("expected SHA256 hex (64 chars), got %d chars", len(hash))
	}
}

func TestFileSource_Fetch_HashChangesOnModify(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	if err := os.WriteFile(path, []byte(validManifestYAML), 0600); err != nil {
		t.Fatal(err)
	}

	fs := NewFileSource(path)
	_, hash1, err := fs.Fetch()
	if err != nil {
		t.Fatalf("first fetch error: %v", err)
	}

	modified := validManifestYAML + "  # comment\n"
	if err := os.WriteFile(path, []byte(modified), 0600); err != nil {
		t.Fatal(err)
	}

	_, hash2, err := fs.Fetch()
	if err != nil {
		t.Fatalf("second fetch error: %v", err)
	}

	if hash1 == hash2 {
		t.Error("expected different hash after file modification")
	}
}

func TestFileSource_Fetch_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte(":::invalid: yaml: [\n"), 0600); err != nil {
		t.Fatal(err)
	}

	fs := NewFileSource(path)
	_, _, err := fs.Fetch()
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestFileSource_Fetch_MissingFile(t *testing.T) {
	fs := NewFileSource("/nonexistent/path/manifest.yaml")
	_, _, err := fs.Fetch()
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}
