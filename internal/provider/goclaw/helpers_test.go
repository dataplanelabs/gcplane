package goclaw

import (
	"testing"
)

func TestStripInternal(t *testing.T) {
	input := map[string]any{
		"id":          "uuid-1",
		"name":        "my-resource",
		"created_at":  "2024-01-01",
		"updated_at":  "2024-01-02",
		"created_by":  "gcplane",
		"owner_id":    "owner-uuid",
		"frontmatter": "some-data",
	}

	result := stripInternal(input)

	// id and name must survive
	if result["id"] != "uuid-1" {
		t.Errorf("expected id to survive stripInternal, got %v", result["id"])
	}
	if result["name"] != "my-resource" {
		t.Errorf("expected name to survive stripInternal, got %v", result["name"])
	}

	// internal fields must be removed
	for _, removed := range []string{"created_at", "updated_at", "created_by", "owner_id", "frontmatter"} {
		if _, ok := result[removed]; ok {
			t.Errorf("expected field %q to be stripped, but it remains", removed)
		}
	}
}

func TestStripInternal_TenantFields(t *testing.T) {
	input := map[string]any{
		"id":           "uuid-1",
		"slug":         "acme",
		"name":         "Acme Corp",
		"tenant_id":    "tenant-123",
		"tenant_name":  "Parent Tenant",
		"tenant_slug":  "parent",
		"created_at":   "2024-01-01",
		"created_by":   "gcplane",
	}

	result := stripInternal(input)

	// core fields must survive
	if result["id"] != "uuid-1" {
		t.Errorf("expected id to survive, got %v", result["id"])
	}
	if result["slug"] != "acme" {
		t.Errorf("expected slug to survive, got %v", result["slug"])
	}
	if result["name"] != "Acme Corp" {
		t.Errorf("expected name to survive, got %v", result["name"])
	}

	// tenant fields must be removed
	for _, removed := range []string{"tenant_id", "tenant_name", "tenant_slug"} {
		if _, ok := result[removed]; ok {
			t.Errorf("expected tenant field %q to be stripped, but it remains", removed)
		}
	}
}

func TestStrVal(t *testing.T) {
	m := map[string]any{
		"str":   "hello",
		"num":   42,
		"empty": "",
	}

	if v := strVal(m, "str"); v != "hello" {
		t.Errorf("expected hello, got %q", v)
	}
	if v := strVal(m, "num"); v != "" {
		t.Errorf("expected empty for non-string, got %q", v)
	}
	if v := strVal(m, "missing"); v != "" {
		t.Errorf("expected empty for missing key, got %q", v)
	}
	if v := strVal(m, "empty"); v != "" {
		t.Errorf("expected empty string, got %q", v)
	}
}

func TestTranslateSpec(t *testing.T) {
	spec := map[string]any{
		"displayName":  "Test",
		"providerType": "openrouter",
		"apiKey":       "secret",
	}

	result := translateSpec(spec)

	if result["display_name"] != "Test" {
		t.Errorf("expected display_name=Test, got %v", result["display_name"])
	}
	if result["provider_type"] != "openrouter" {
		t.Errorf("expected provider_type=openrouter, got %v", result["provider_type"])
	}
	if result["api_key"] != "secret" {
		t.Errorf("expected api_key=secret, got %v", result["api_key"])
	}
}

func TestTranslateResult(t *testing.T) {
	raw := map[string]any{
		"display_name":  "Test",
		"provider_type": "openrouter",
		"api_key":       "***",
	}

	result := translateResult(raw)

	if result["displayName"] != "Test" {
		t.Errorf("expected displayName=Test, got %v", result["displayName"])
	}
	if result["providerType"] != "openrouter" {
		t.Errorf("expected providerType=openrouter, got %v", result["providerType"])
	}
}

func TestCopyMap(t *testing.T) {
	orig := map[string]any{"a": 1, "b": "two"}
	cp := copyMap(orig)

	if cp["a"] != orig["a"] || cp["b"] != orig["b"] {
		t.Error("copy does not match original")
	}

	// mutations to copy must not affect original
	cp["a"] = 99
	if orig["a"] == 99 {
		t.Error("copyMap is not a proper shallow copy")
	}
}

func TestStripInternal_AgentConfigFields(t *testing.T) {
	input := map[string]any{
		"id":                  "uuid-1",
		"agent_key":           "my-agent",
		"context_window":      200000,
		"max_tool_iterations": 50,
		"tools_config":        map[string]any{"profile": "coding"},
		"sandbox_config":      map[string]any{},
		"other_config":        map[string]any{},
		"created_at":          "2024-01-01",
	}

	result := stripInternal(input)

	// context_window and max_tool_iterations are now observable (removed from internalFields)
	if result["context_window"] != 200000 {
		t.Errorf("expected context_window to survive, got %v", result["context_window"])
	}
	if result["max_tool_iterations"] != 50 {
		t.Errorf("expected max_tool_iterations to survive, got %v", result["max_tool_iterations"])
	}

	// Complex JSONB configs should still be stripped
	for _, f := range []string{"tools_config", "sandbox_config", "other_config"} {
		if _, ok := result[f]; ok {
			t.Errorf("expected JSONB config %q to be stripped", f)
		}
	}

	// Timestamps should still be stripped
	if _, ok := result["created_at"]; ok {
		t.Error("expected created_at to be stripped")
	}
}

func TestStrVal_NilMap(t *testing.T) {
	// should not panic on nil map
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("strVal panicked on nil map: %v", r)
		}
	}()
	v := strVal(nil, "key")
	if v != "" {
		t.Errorf("expected empty string, got %q", v)
	}
}
