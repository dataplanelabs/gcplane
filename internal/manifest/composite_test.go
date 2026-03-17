package manifest

import (
	"strings"
	"testing"
)

func makeCompositeManifest(definition, instance string) *Manifest {
	return &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{
				Kind: "CompositeDefinition",
				Name: "ChatBot",
				Spec: map[string]any{
					"resources": []any{
						map[string]any{
							"kind": "Agent",
							"name": "{{ .name }}",
							"spec": map[string]any{
								"displayName": "{{ .displayName }}",
								"provider":    "{{ .provider }}",
							},
						},
						map[string]any{
							"kind": "Channel",
							"name": "{{ .name }}-channel",
							"spec": map[string]any{
								"channelType": "{{ .channelType }}",
								"agentKey":    "{{ .name }}",
							},
						},
					},
				},
			},
			{
				Kind: ResourceKind("ChatBot"),
				Name: "support-bot",
				Spec: map[string]any{
					"displayName": "Support Bot",
					"provider":    "anthropic",
					"channelType": "telegram",
				},
			},
		},
	}
}

func TestExpandComposites_SimpleExpansion(t *testing.T) {
	m := makeCompositeManifest("", "")

	if err := ExpandComposites(m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(m.Resources) != 2 {
		t.Fatalf("expected 2 expanded resources, got %d", len(m.Resources))
	}

	agent := m.Resources[0]
	if agent.Kind != KindAgent {
		t.Errorf("expected Agent, got %q", agent.Kind)
	}
	if agent.Name != "support-bot" {
		t.Errorf("expected name %q, got %q", "support-bot", agent.Name)
	}
	if agent.Spec["displayName"] != "Support Bot" {
		t.Errorf("expected displayName 'Support Bot', got %v", agent.Spec["displayName"])
	}

	channel := m.Resources[1]
	if channel.Kind != KindChannel {
		t.Errorf("expected Channel, got %q", channel.Kind)
	}
	if channel.Name != "support-bot-channel" {
		t.Errorf("expected name %q, got %q", "support-bot-channel", channel.Name)
	}
}

func TestExpandComposites_MultipleInstances(t *testing.T) {
	m := &Manifest{
		Resources: []Resource{
			{
				Kind: "CompositeDefinition",
				Name: "SimpleAgent",
				Spec: map[string]any{
					"resources": []any{
						map[string]any{
							"kind": "Agent",
							"name": "{{ .name }}",
							"spec": map[string]any{
								"model": "{{ .model }}",
							},
						},
					},
				},
			},
			{
				Kind: ResourceKind("SimpleAgent"),
				Name: "bot-a",
				Spec: map[string]any{"model": "claude-3"},
			},
			{
				Kind: ResourceKind("SimpleAgent"),
				Name: "bot-b",
				Spec: map[string]any{"model": "gpt-4"},
			},
		},
	}

	if err := ExpandComposites(m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(m.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(m.Resources))
	}
	if m.Resources[0].Name != "bot-a" {
		t.Errorf("expected bot-a, got %q", m.Resources[0].Name)
	}
	if m.Resources[1].Name != "bot-b" {
		t.Errorf("expected bot-b, got %q", m.Resources[1].Name)
	}
}

func TestExpandComposites_LabelsPropagate(t *testing.T) {
	m := &Manifest{
		Resources: []Resource{
			{
				Kind: "CompositeDefinition",
				Name: "SimpleAgent",
				Spec: map[string]any{
					"resources": []any{
						map[string]any{
							"kind": "Agent",
							"name": "{{ .name }}",
							"spec": map[string]any{"model": "x"},
						},
					},
				},
			},
			{
				Kind:   ResourceKind("SimpleAgent"),
				Name:   "my-bot",
				Labels: map[string]string{"env": "prod", "team": "platform"},
				Spec:   map[string]any{},
			},
		},
	}

	if err := ExpandComposites(m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(m.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(m.Resources))
	}
	res := m.Resources[0]
	if res.Labels["env"] != "prod" {
		t.Errorf("expected label env=prod, got %v", res.Labels)
	}
	if res.Labels["team"] != "platform" {
		t.Errorf("expected label team=platform, got %v", res.Labels)
	}
}

func TestExpandComposites_NoDefinitions_NoChange(t *testing.T) {
	m := &Manifest{
		Resources: []Resource{
			{Kind: KindProvider, Name: "anthropic", Spec: map[string]any{"type": "anthropic"}},
		},
	}

	if err := ExpandComposites(m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(m.Resources) != 1 {
		t.Errorf("expected 1 resource unchanged, got %d", len(m.Resources))
	}
	if m.Resources[0].Kind != KindProvider {
		t.Errorf("expected Provider unchanged, got %q", m.Resources[0].Kind)
	}
}

func TestExpandComposites_InvalidTemplate_ReturnsError(t *testing.T) {
	m := &Manifest{
		Resources: []Resource{
			{
				Kind: "CompositeDefinition",
				Name: "BadDef",
				Spec: map[string]any{
					"resources": []any{
						map[string]any{
							"kind": "Agent",
							"name": "{{ .name }",  // malformed template
							"spec": map[string]any{},
						},
					},
				},
			},
			{
				Kind: ResourceKind("BadDef"),
				Name: "instance",
				Spec: map[string]any{},
			},
		},
	}

	err := ExpandComposites(m)
	if err == nil {
		t.Fatal("expected error for invalid template")
	}
	if !strings.Contains(err.Error(), "expand") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExpandComposites_InvalidResourcesField_ReturnsError(t *testing.T) {
	m := &Manifest{
		Resources: []Resource{
			{
				Kind: "CompositeDefinition",
				Name: "BadDef",
				Spec: map[string]any{
					"resources": "not-an-array",
				},
			},
		},
	}

	err := ExpandComposites(m)
	if err == nil {
		t.Fatal("expected error for non-array resources field")
	}
	if !strings.Contains(err.Error(), "must be an array") {
		t.Errorf("unexpected error message: %v", err)
	}
}
