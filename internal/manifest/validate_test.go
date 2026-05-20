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
			{Kind: KindAgentTeam, Name: "team", Spec: map[string]any{
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
			{Kind: KindAgentTeam, Name: "team", Spec: map[string]any{"lead": "ghost"}},
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

func TestValidateReferences_BrokenAgentLink(t *testing.T) {
	// Spec values match name halves but agents are not declared.
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{Kind: KindAgentLink, Name: "ghost--also-ghost", Spec: map[string]any{
				"sourceAgent": "ghost",
				"targetAgent": "also-ghost",
			}},
		},
	}
	errs := Validate(m)
	hasSrc, hasTgt := false, false
	for _, e := range errs {
		if strings.Contains(e.Error(), "sourceAgent") && strings.Contains(e.Error(), "ghost") {
			hasSrc = true
		}
		if strings.Contains(e.Error(), "targetAgent") && strings.Contains(e.Error(), "also-ghost") {
			hasTgt = true
		}
	}
	if !hasSrc || !hasTgt {
		t.Errorf("expected ref errors for both sourceAgent and targetAgent, got: %v", errs)
	}
}

func TestValidateReferences_AgentLinkSpecNameMismatch(t *testing.T) {
	// Composite name says "planner--coder" but spec.sourceAgent says "ghost".
	// Without the name-match rule, the create call would silently use the name.
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{Kind: KindProvider, Name: "p", Spec: map[string]any{}},
			{Kind: KindAgent, Name: "planner", Spec: map[string]any{"provider": "p"}},
			{Kind: KindAgent, Name: "coder", Spec: map[string]any{"provider": "p"}},
			{Kind: KindAgentLink, Name: "planner--coder", Spec: map[string]any{
				"sourceAgent": "ghost", // mismatch
				"targetAgent": "coder",
			}},
		},
	}
	errs := Validate(m)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "must match name prefix") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected name-prefix mismatch error, got: %v", errs)
	}
}

func TestValidateReferences_AgentLinkBadName(t *testing.T) {
	// 3-segment name should be rejected by the validator.
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{Kind: KindAgentLink, Name: "a--b--c", Spec: map[string]any{}},
		},
	}
	errs := Validate(m)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "invalid AgentLink name") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected invalid-name error for 3-segment composite, got: %v", errs)
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

func TestValidate_NewKinds(t *testing.T) {
	newKinds := []struct {
		kind     ResourceKind
		name     string
		testName string
	}{
		{KindTenant, "my-tenant", "KindTenant"},
		{KindBuiltinToolConfig, "tool-config", "KindBuiltinToolConfig"},
		{KindSkillConfig, "skill-config", "KindSkillConfig"},
		{KindMCPCredentials, "mcp-creds", "KindMCPCredentials"},
	}

	for _, tc := range newKinds {
		t.Run(tc.testName, func(t *testing.T) {
			m := &Manifest{
				APIVersion: "gcplane.io/v1",
				Kind:       "Manifest",
				Resources: []Resource{
					{Kind: tc.kind, Name: tc.name, Spec: map[string]any{"x": 1}},
				},
			}

			errs := Validate(m)
			if len(errs) != 0 {
				t.Errorf("expected no errors for %s, got %d: %v", tc.testName, len(errs), errs)
			}
		})
	}
}

func TestValidate_SkillSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    map[string]any
		wantErr string
	}{
		{"valid full spec", map[string]any{"visibility": "tenant", "status": "active", "tags": []any{"a", "b"}}, ""},
		{"empty spec ok", map[string]any{}, ""},
		{"bad visibility", map[string]any{"visibility": "weird"}, "visibility"},
		{"bad status", map[string]any{"status": "frozen"}, "status"},
		{"tags not list", map[string]any{"tags": "single-string"}, "tags must be a list"},
		{"tags duplicate", map[string]any{"tags": []any{"x", "x"}}, "duplicate"},
		{"tags non-string", map[string]any{"tags": []any{"ok", 42}}, "must be a string"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &Manifest{
				APIVersion: "gcplane.io/v1",
				Kind:       "Manifest",
				Resources: []Resource{
					{Kind: KindSkill, Name: "my-skill", Spec: tc.spec},
				},
			}
			errs := Validate(m)
			if tc.wantErr == "" {
				if len(errs) != 0 {
					t.Errorf("expected no errors, got: %v", errs)
				}
				return
			}
			joined := ""
			for _, e := range errs {
				joined += e.Error() + "\n"
			}
			if !strings.Contains(joined, tc.wantErr) {
				t.Errorf("expected error containing %q, got: %s", tc.wantErr, joined)
			}
		})
	}
}
