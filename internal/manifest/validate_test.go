package manifest

import (
	"strings"
	"testing"
)

func TestValidate_Valid(t *testing.T) {
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{Kind: KindProvider, Name: "anthropic", Spec: map[string]any{"name": "test"}},
			{Kind: KindAgent, Name: "my-bot", Spec: map[string]any{"model": "test"}},
		},
	}

	errs := Validate(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestValidate_BadAPIVersion(t *testing.T) {
	m := &Manifest{APIVersion: "v2", Kind: "Manifest"}
	errs := Validate(m)
	if len(errs) == 0 {
		t.Error("expected error for bad apiVersion")
	}
}

func TestValidate_BadKind(t *testing.T) {
	m := &Manifest{APIVersion: "gcplane.io/v1", Kind: "Config"}
	errs := Validate(m)
	if len(errs) == 0 {
		t.Error("expected error for bad kind")
	}
}

func TestValidate_InvalidKey(t *testing.T) {
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{Kind: KindAgent, Name: "My_Bot", Spec: map[string]any{"x": 1}},
		},
	}

	errs := Validate(m)
	if len(errs) == 0 {
		t.Error("expected error for non-kebab-case key")
	}
}

func TestValidate_DuplicateResource(t *testing.T) {
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{Kind: KindAgent, Name: "bot", Spec: map[string]any{"x": 1}},
			{Kind: KindAgent, Name: "bot", Spec: map[string]any{"x": 2}},
		},
	}

	errs := Validate(m)
	if len(errs) == 0 {
		t.Error("expected error for duplicate resource")
	}
}

func TestValidate_MissingSpec(t *testing.T) {
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{Kind: KindAgent, Name: "bot"},
		},
	}

	errs := Validate(m)
	if len(errs) == 0 {
		t.Error("expected error for missing spec")
	}
}

// --- Reference validation tests ---

func TestValidateReferences_ValidRefs(t *testing.T) {
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{Kind: KindProvider, Name: "anthropic", Spec: map[string]any{"type": "anthropic"}},
			{Kind: KindAgent, Name: "bot", Spec: map[string]any{"provider": "anthropic"}},
			{Kind: KindChannel, Name: "ch", Spec: map[string]any{"agentKey": "bot"}},
			{Kind: KindCronJob, Name: "job", Spec: map[string]any{"agentKey": "bot"}},
			{Kind: KindMCPServer, Name: "mcp", Spec: map[string]any{
				"grants": map[string]any{"agents": []any{"bot"}},
			}},
			{Kind: KindTeam, Name: "team", Spec: map[string]any{
				"lead":    "bot",
				"members": []any{"bot"},
			}},
		},
	}
	errs := Validate(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid refs, got: %v", errs)
	}
}

func TestValidateReferences_BrokenProvider(t *testing.T) {
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{Kind: KindAgent, Name: "bot", Spec: map[string]any{"provider": "nonexistent"}},
		},
	}
	errs := Validate(m)
	if len(errs) == 0 {
		t.Fatal("expected reference error")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "nonexistent") && strings.Contains(e.Error(), "Provider") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about missing Provider, got: %v", errs)
	}
}

func TestValidateReferences_BrokenChannelAgent(t *testing.T) {
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{Kind: KindChannel, Name: "ch", Spec: map[string]any{"agentKey": "ghost"}},
		},
	}
	errs := Validate(m)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "ghost") && strings.Contains(e.Error(), "Agent") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about missing Agent for Channel, got: %v", errs)
	}
}

func TestValidateReferences_BrokenCronJobAgent(t *testing.T) {
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{Kind: KindCronJob, Name: "job", Spec: map[string]any{"agentKey": "ghost"}},
		},
	}
	errs := Validate(m)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "ghost") && strings.Contains(e.Error(), "Agent") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about missing Agent for CronJob, got: %v", errs)
	}
}

func TestValidateReferences_BrokenMCPServerGrant(t *testing.T) {
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{Kind: KindMCPServer, Name: "mcp", Spec: map[string]any{
				"grants": map[string]any{"agents": []any{"ghost"}},
			}},
		},
	}
	errs := Validate(m)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "ghost") && strings.Contains(e.Error(), "Agent") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about missing Agent in MCPServer grants, got: %v", errs)
	}
}

func TestValidateReferences_BrokenTeamLead(t *testing.T) {
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{Kind: KindTeam, Name: "team", Spec: map[string]any{"lead": "ghost"}},
		},
	}
	errs := Validate(m)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "ghost") && strings.Contains(e.Error(), "Agent") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about missing Agent for Team lead, got: %v", errs)
	}
}

func TestValidateReferences_MultipleErrors(t *testing.T) {
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{Kind: KindAgent, Name: "bot", Spec: map[string]any{"provider": "missing-provider"}},
			{Kind: KindChannel, Name: "ch", Spec: map[string]any{"agentKey": "missing-agent"}},
		},
	}
	errs := Validate(m)
	if len(errs) < 2 {
		t.Errorf("expected at least 2 reference errors, got %d: %v", len(errs), errs)
	}
}

func TestApplyOrder_ContainsAllKinds(t *testing.T) {
	order := ApplyOrder()
	if len(order) != len(validKinds) {
		t.Errorf("ApplyOrder has %d kinds, validKinds has %d", len(order), len(validKinds))
	}

	for _, k := range order {
		if !validKinds[k] {
			t.Errorf("ApplyOrder contains unknown kind %s", k)
		}
	}
}
