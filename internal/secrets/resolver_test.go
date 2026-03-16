package secrets

import (
	"os"
	"testing"
)

func TestResolveEnvVars(t *testing.T) {
	os.Setenv("TEST_GCPLANE_KEY", "my-secret-key")
	defer os.Unsetenv("TEST_GCPLANE_KEY")

	result, err := ResolveEnvVars("Bearer ${TEST_GCPLANE_KEY}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Bearer my-secret-key" {
		t.Errorf("expected 'Bearer my-secret-key', got %q", result)
	}
}

func TestResolveEnvVars_Missing(t *testing.T) {
	_, err := ResolveEnvVars("${NONEXISTENT_VAR_GCPLANE}")
	if err == nil {
		t.Error("expected error for missing env var")
	}
}

func TestResolveFileRef(t *testing.T) {
	tmp, err := os.CreateTemp("", "gcplane-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	tmp.WriteString("  secret-value  \n")
	tmp.Close()

	result, err := ResolveFileRef("file://" + tmp.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "secret-value" {
		t.Errorf("expected 'secret-value', got %q", result)
	}
}

func TestResolveFileRef_NotAFileRef(t *testing.T) {
	result, err := ResolveFileRef("just-a-string")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "just-a-string" {
		t.Errorf("expected passthrough, got %q", result)
	}
}

func TestResolve_FileRefTakesPrecedence(t *testing.T) {
	tmp, err := os.CreateTemp("", "gcplane-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	tmp.WriteString("file-content")
	tmp.Close()

	result, err := Resolve("file://" + tmp.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "file-content" {
		t.Errorf("expected 'file-content', got %q", result)
	}
}
