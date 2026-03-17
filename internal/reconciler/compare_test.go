package reconciler

import (
	"testing"
)

func TestCompareSpec_Identical(t *testing.T) {
	desired := map[string]any{"name": "test", "model": "gpt-4"}
	actual := map[string]any{"name": "test", "model": "gpt-4"}

	diffs := CompareSpec(desired, actual)
	if len(diffs) != 0 {
		t.Errorf("expected no diffs, got %d: %v", len(diffs), diffs)
	}
}

func TestCompareSpec_NewField(t *testing.T) {
	desired := map[string]any{"name": "test", "model": "gpt-4"}
	actual := map[string]any{"name": "test"}

	diffs := CompareSpec(desired, actual)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d: %v", len(diffs), diffs)
	}
	if _, ok := diffs["model"]; !ok {
		t.Error("expected diff for 'model'")
	}
}

func TestCompareSpec_ChangedField(t *testing.T) {
	desired := map[string]any{"model": "claude-sonnet-4-20250514"}
	actual := map[string]any{"model": "claude-haiku-4-5-20251001"}

	diffs := CompareSpec(desired, actual)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	d := diffs["model"]
	if d.Old != "claude-haiku-4-5-20251001" || d.New != "claude-sonnet-4-20250514" {
		t.Errorf("unexpected diff: %+v", d)
	}
}

func TestCompareSpec_APImaskedSecretSkipped(t *testing.T) {
	// API returns "***" for a field it masks (e.g. apiKey on Provider).
	// CompareSpec should still skip those to avoid false drift.
	desired := map[string]any{"apiKey": "sk-real-key", "name": "test"}
	actual := map[string]any{"apiKey": "***", "name": "test"}

	diffs := CompareSpec(desired, actual)
	if len(diffs) != 0 {
		t.Errorf("expected api-masked field skipped, got diffs: %v", diffs)
	}
}

func TestCompareSpecExcluding_SkipsExcludedFields(t *testing.T) {
	desired := map[string]any{"agentKey": "my-agent", "name": "slack", "channel": "#general"}
	actual := map[string]any{"name": "slack", "channel": "#general"}

	diffs := CompareSpecExcluding(desired, actual, []string{"agentKey"})
	if len(diffs) != 0 {
		t.Errorf("expected excluded field skipped, got diffs: %v", diffs)
	}
}

func TestCompareSpecExcluding_DetectsDriftOnNonExcluded(t *testing.T) {
	desired := map[string]any{"agentKey": "my-agent", "channel": "#new"}
	actual := map[string]any{"channel": "#old"}

	diffs := CompareSpecExcluding(desired, actual, []string{"agentKey"})
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d: %v", len(diffs), diffs)
	}
	if _, ok := diffs["channel"]; !ok {
		t.Error("expected diff for 'channel'")
	}
}

func TestCompareSpec_NestedMap(t *testing.T) {
	desired := map[string]any{
		"config": map[string]any{"timeout": 30, "retry": true},
	}
	actual := map[string]any{
		"config": map[string]any{"timeout": 10, "retry": true},
	}

	diffs := CompareSpec(desired, actual)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d: %v", len(diffs), diffs)
	}
	if _, ok := diffs["config.timeout"]; !ok {
		t.Error("expected diff for 'config.timeout'")
	}
}

func TestCompareSpec_NumericTypeEquality(t *testing.T) {
	// YAML gives int, JSON gives float64 — should be equal
	desired := map[string]any{"port": 8080}
	actual := map[string]any{"port": float64(8080)}

	diffs := CompareSpec(desired, actual)
	if len(diffs) != 0 {
		t.Errorf("expected numeric types to match, got diffs: %v", diffs)
	}
}
