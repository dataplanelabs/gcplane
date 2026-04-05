package manifest

import (
	"testing"
)

func TestParseIgnoreFields_Empty(t *testing.T) {
	fields := ParseIgnoreFields(nil)
	if len(fields) != 0 {
		t.Errorf("expected nil, got %v", fields)
	}

	fields = ParseIgnoreFields(map[string]string{})
	if len(fields) != 0 {
		t.Errorf("expected nil for empty map, got %v", fields)
	}
}

func TestParseIgnoreFields_Single(t *testing.T) {
	fields := ParseIgnoreFields(map[string]string{
		AnnotationIgnoreFields: "apiKey",
	})
	if len(fields) != 1 || fields[0] != "apiKey" {
		t.Errorf("expected [apiKey], got %v", fields)
	}
}

func TestParseIgnoreFields_Multiple(t *testing.T) {
	fields := ParseIgnoreFields(map[string]string{
		AnnotationIgnoreFields: "apiKey, systemPrompt, contextFiles",
	})
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d: %v", len(fields), fields)
	}
	expected := []string{"apiKey", "systemPrompt", "contextFiles"}
	for i, f := range fields {
		if f != expected[i] {
			t.Errorf("field[%d]: expected %q, got %q", i, expected[i], f)
		}
	}
}

func TestParseIgnoreFields_TrimsWhitespace(t *testing.T) {
	fields := ParseIgnoreFields(map[string]string{
		AnnotationIgnoreFields: " apiKey ,  systemPrompt ",
	})
	if len(fields) != 2 {
		t.Fatalf("expected 2 fields, got %d: %v", len(fields), fields)
	}
	if fields[0] != "apiKey" || fields[1] != "systemPrompt" {
		t.Errorf("expected trimmed values, got %v", fields)
	}
}

func TestParseIgnoreFields_SkipsEmpty(t *testing.T) {
	fields := ParseIgnoreFields(map[string]string{
		AnnotationIgnoreFields: "apiKey,,, systemPrompt",
	})
	if len(fields) != 2 {
		t.Errorf("expected 2 fields (skipping empty), got %d: %v", len(fields), fields)
	}
}
