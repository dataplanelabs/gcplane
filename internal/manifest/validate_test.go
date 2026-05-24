package manifest

import (
	"os"
	"path/filepath"
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

func TestValidateSkillSpec_SourceDirMissing(t *testing.T) {
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{Kind: KindSkill, Name: "gh-read", Spec: map[string]any{
				"sourceDir": "/nonexistent/path/skills/gh-read",
			}},
		},
	}
	errs := Validate(m)
	if len(errs) == 0 {
		t.Fatal("expected error for missing sourceDir")
	}
	found := false
	for _, err := range errs {
		if strings.Contains(err.Error(), "sourceDir") && strings.Contains(err.Error(), "/nonexistent/path") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected sourceDir error citing path, got: %v", errs)
	}
}

func TestValidateSkillSpec_SourceDirNoSkillMd(t *testing.T) {
	dir := t.TempDir()
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{Kind: KindSkill, Name: "no-md", Spec: map[string]any{"sourceDir": dir}},
		},
	}
	errs := Validate(m)
	found := false
	for _, err := range errs {
		if strings.Contains(err.Error(), "SKILL.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SKILL.md error, got: %v", errs)
	}
}

func TestValidateSkillSpec_OversizeFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: x\n---\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	big := make([]byte, (4*1024*1024)+1)
	if err := os.WriteFile(filepath.Join(dir, "big.bin"), big, 0o644); err != nil {
		t.Fatalf("write big.bin: %v", err)
	}
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{Kind: KindSkill, Name: "oversize", Spec: map[string]any{"sourceDir": dir}},
		},
	}
	errs := Validate(m)
	found := false
	for _, err := range errs {
		if strings.Contains(err.Error(), "big.bin") && strings.Contains(err.Error(), "exceeds") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected oversize error for big.bin, got: %v", errs)
	}
}

func TestValidateSkillSpec_SkipsHiddenAndGitFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: x\n---\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git", "objects", "pack"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	bigPack := make([]byte, (4*1024*1024)+1)
	if err := os.WriteFile(filepath.Join(dir, ".git", "objects", "pack", "pack.bin"), bigPack, 0o644); err != nil {
		t.Fatalf("write pack: %v", err)
	}
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{Kind: KindSkill, Name: "with-git", Spec: map[string]any{"sourceDir": dir}},
		},
	}
	errs := Validate(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors (validate must skip .git like skillpkg does), got: %v", errs)
	}
}

func TestValidateSkillSpec_HappyPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: x\n---\nbody\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{Kind: KindSkill, Name: "happy", Spec: map[string]any{"sourceDir": dir}},
		},
	}
	errs := Validate(m)
	if len(errs) != 0 {
		t.Errorf("expected zero errors on happy path, got: %v", errs)
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

func TestNormalizeSemver(t *testing.T) {
	cases := map[string]string{
		"2.71":        "v2.71",
		"v2.71.0":     "v2.71.0",
		"2.71.0-rc1":  "v2.71.0-rc1",
		"":            "",
		"  1.2.3  ":   "v1.2.3",
		"V0.1":        "V0.1",
	}
	for in, want := range cases {
		got := normalizeSemver(in)
		if got != want {
			t.Errorf("normalizeSemver(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestValidateCliConstraintShape(t *testing.T) {
	ok := []string{">=2.50", ">=2.50.0", ">=v2.50", ">= 2.50", ">=2.50.0-rc1"}
	for _, s := range ok {
		if err := validateCliConstraintShape(s); err != nil {
			t.Errorf("expected %q to validate, got: %v", s, err)
		}
	}
	bad := []string{
		"",
		"^2.50",
		"~2.50",
		">=2.50, <3",
		"2.50",
		">=abc",
		">=",
	}
	for _, s := range bad {
		if err := validateCliConstraintShape(s); err == nil {
			t.Errorf("expected %q to fail, got nil", s)
		}
	}
}

func TestValidateSkillSpec_RequiresCli_UnsupportedTilde(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: x\n---\n"), 0o644)
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{{
			Kind: KindSkill, Name: "t", Spec: map[string]any{
				"sourceDir": dir,
				"requires":  map[string]any{"cli": map[string]any{"gh": "~2.50"}},
			},
		}},
	}
	errs := Validate(m)
	joined := ""
	for _, e := range errs {
		joined += e.Error() + "\n"
	}
	if !strings.Contains(joined, "unsupported constraint shape") {
		t.Errorf("expected unsupported-constraint error for ~2.50, got: %s", joined)
	}
}

func TestValidateSkillSpec_RequiresCli_UnsupportedCaret(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: x\n---\n"), 0o644)
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{{
			Kind: KindSkill, Name: "t", Spec: map[string]any{
				"sourceDir": dir,
				"requires":  map[string]any{"cli": map[string]any{"gh": "^2.50"}},
			},
		}},
	}
	errs := Validate(m)
	joined := ""
	for _, e := range errs {
		joined += e.Error() + "\n"
	}
	if !strings.Contains(joined, "unsupported constraint shape") {
		t.Errorf("expected unsupported-constraint error for ^2.50, got: %s", joined)
	}
}

func TestValidateSkillSpec_RequiresCli_HappyPath(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: x\n---\n"), 0o644)
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{{
			Kind: KindSkill, Name: "t", Spec: map[string]any{
				"sourceDir": dir,
				"requires":  map[string]any{"cli": map[string]any{"gh": ">=2.50", "kubectl": ">=1.30"}},
			},
		}},
	}
	errs := Validate(m)
	for _, e := range errs {
		if strings.Contains(e.Error(), "requires.cli") {
			t.Errorf("unexpected requires.cli error: %v", e)
		}
	}
}
