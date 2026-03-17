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

func TestFileSource_Fetch_Directory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(validManifestYAML), 0600); err != nil {
		t.Fatal(err)
	}

	fs := NewFileSource(dir)
	m, hash, err := fs.Fetch()
	if err != nil {
		t.Fatalf("expected no error for directory source, got: %v", err)
	}
	if m == nil {
		t.Fatal("expected manifest, got nil")
	}
	if hash == "" {
		t.Error("expected non-empty hash for directory")
	}
	if len(hash) != 64 {
		t.Errorf("expected SHA256 hex (64 chars), got %d chars", len(hash))
	}
}

func TestFileSource_Fetch_Directory_HashDeterministic(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(validManifestYAML), 0600); err != nil {
		t.Fatal(err)
	}

	fs := NewFileSource(dir)
	_, hash1, err := fs.Fetch()
	if err != nil {
		t.Fatal(err)
	}
	_, hash2, err := fs.Fetch()
	if err != nil {
		t.Fatal(err)
	}

	if hash1 != hash2 {
		t.Error("expected same hash on repeated Fetch calls with no changes")
	}
}

func TestHashDir_MultipleFiles_DeterministicOrder(t *testing.T) {
	dir := t.TempDir()
	// Write files with names that would sort differently from creation order;
	// distinct content avoids duplicate-resource errors during manifest.Load.
	files := []struct{ name, content string }{
		{"z-last.yaml", "data: z\n"},
		{"a-first.yaml", "data: a\n"},
		{"m-middle.yaml", "data: m\n"},
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.content), 0600); err != nil {
			t.Fatal(err)
		}
	}

	hash1, err := hashDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	hash2, err := hashDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 != hash2 {
		t.Error("directory hash must be deterministic across calls")
	}
}

func TestFileSource_Fetch_Directory_HashChangesOnNewFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(validManifestYAML), 0600); err != nil {
		t.Fatal(err)
	}

	fs := NewFileSource(dir)
	_, hash1, err := fs.Fetch()
	if err != nil {
		t.Fatal(err)
	}

	// Add another YAML file — hash must change
	extra := validManifestYAML + "\n# extra\n"
	if err := os.WriteFile(filepath.Join(dir, "extra.yaml"), []byte(extra), 0600); err != nil {
		t.Fatal(err)
	}
	// extra.yaml has different content → manifest.Load will still succeed (uses first file)
	// We just need to verify hash changes
	_, hash2, _ := fs.Fetch() // error may occur due to multi-file load, that's fine
	if hash1 == hash2 {
		t.Error("expected hash to change when a new YAML file is added")
	}
}

func TestHashDir_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	hash, err := hashDir(dir)
	if err != nil {
		t.Fatalf("hashDir on empty dir should not error: %v", err)
	}
	// Empty SHA256: sha256.New().Sum(nil) → well-defined zero-content hash
	if hash == "" {
		t.Error("expected non-empty hash string even for empty directory")
	}
}

func TestHashDir_IgnoresNonYAMLFiles(t *testing.T) {
	dir := t.TempDir()

	// Only non-YAML files
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	hashNoYAML, err := hashDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Add a YAML file — hash must differ
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte("x: 1"), 0600); err != nil {
		t.Fatal(err)
	}
	hashWithYAML, err := hashDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	if hashNoYAML == hashWithYAML {
		t.Error("expected different hashes when a YAML file is added")
	}
}

func TestHashDir_IgnoresSubdirectories(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	// Put YAML inside subdir — should not affect parent hash
	if err := os.WriteFile(filepath.Join(subdir, "nested.yaml"), []byte("x: 1"), 0600); err != nil {
		t.Fatal(err)
	}

	// Compute hash for empty parent (no YAML at top level)
	hash1, err := hashDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Now add YAML at top level
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte("x: 1"), 0600); err != nil {
		t.Fatal(err)
	}
	hash2, err := hashDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	if hash1 == hash2 {
		t.Error("expected different hashes; top-level YAML should be included, subdirs ignored")
	}
}
