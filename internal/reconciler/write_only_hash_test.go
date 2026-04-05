package reconciler

import (
	"testing"
)

func TestHashWriteOnlyFields_Basic(t *testing.T) {
	spec := map[string]any{
		"apiKey":      "sk-real-key",
		"displayName": "Anthropic",
	}
	hash := HashWriteOnlyFields(spec, []string{"apiKey"}, nil)
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if len(hash) != 64 { // SHA-256 hex length
		t.Errorf("expected 64-char hex hash, got %d chars: %s", len(hash), hash)
	}
}

func TestHashWriteOnlyFields_Deterministic(t *testing.T) {
	spec := map[string]any{
		"apiKey":       "sk-key-1",
		"contextFiles": []any{map[string]any{"IDENTITY.md": "content"}},
		"systemPrompt": "Be helpful",
	}
	woFields := []string{"contextFiles", "systemPrompt", "apiKey"}

	hash1 := HashWriteOnlyFields(spec, woFields, nil)
	hash2 := HashWriteOnlyFields(spec, woFields, nil)
	if hash1 != hash2 {
		t.Errorf("hash not deterministic: %s != %s", hash1, hash2)
	}
}

func TestHashWriteOnlyFields_DifferentValues(t *testing.T) {
	spec1 := map[string]any{"apiKey": "key-1"}
	spec2 := map[string]any{"apiKey": "key-2"}

	hash1 := HashWriteOnlyFields(spec1, []string{"apiKey"}, nil)
	hash2 := HashWriteOnlyFields(spec2, []string{"apiKey"}, nil)
	if hash1 == hash2 {
		t.Error("expected different hashes for different values")
	}
}

func TestHashWriteOnlyFields_IgnoreFields(t *testing.T) {
	spec := map[string]any{
		"apiKey":       "sk-key",
		"contextFiles": []any{"file1"},
		"systemPrompt": "prompt",
	}
	woFields := []string{"apiKey", "contextFiles", "systemPrompt"}

	hashAll := HashWriteOnlyFields(spec, woFields, nil)
	hashIgnoreOne := HashWriteOnlyFields(spec, woFields, []string{"apiKey"})
	if hashAll == hashIgnoreOne {
		t.Error("expected different hash when ignoring a field")
	}

	// Ignoring all fields should return empty string
	hashIgnoreAll := HashWriteOnlyFields(spec, woFields, []string{"apiKey", "contextFiles", "systemPrompt"})
	if hashIgnoreAll != "" {
		t.Errorf("expected empty hash when all fields ignored, got %s", hashIgnoreAll)
	}
}

func TestHashWriteOnlyFields_NoWriteOnlyFields(t *testing.T) {
	spec := map[string]any{"displayName": "Test"}
	hash := HashWriteOnlyFields(spec, nil, nil)
	if hash != "" {
		t.Errorf("expected empty hash for no write-only fields, got %s", hash)
	}
}

func TestHashWriteOnlyFields_MissingFieldsInSpec(t *testing.T) {
	spec := map[string]any{"displayName": "Test"}
	hash := HashWriteOnlyFields(spec, []string{"apiKey", "contextFiles"}, nil)
	if hash != "" {
		t.Errorf("expected empty hash when no write-only fields present in spec, got %s", hash)
	}
}

func TestHashWriteOnlyFields_PartialFieldsPresent(t *testing.T) {
	spec := map[string]any{
		"apiKey":      "sk-key",
		"displayName": "Test",
	}
	// contextFiles is in write-only list but not in spec — should hash only apiKey
	hash := HashWriteOnlyFields(spec, []string{"apiKey", "contextFiles"}, nil)
	hashOnlyKey := HashWriteOnlyFields(spec, []string{"apiKey"}, nil)
	if hash != hashOnlyKey {
		t.Error("expected same hash when missing fields are ignored")
	}
}
