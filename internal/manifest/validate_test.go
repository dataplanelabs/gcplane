package manifest

import (
	"testing"
)

func TestValidate_Valid(t *testing.T) {
	m := &Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Resources: []Resource{
			{Kind: KindProvider, Key: "anthropic", Spec: map[string]any{"name": "test"}},
			{Kind: KindAgent, Key: "my-bot", Spec: map[string]any{"model": "test"}},
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
			{Kind: KindAgent, Key: "My_Bot", Spec: map[string]any{"x": 1}},
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
			{Kind: KindAgent, Key: "bot", Spec: map[string]any{"x": 1}},
			{Kind: KindAgent, Key: "bot", Spec: map[string]any{"x": 2}},
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
			{Kind: KindAgent, Key: "bot"},
		},
	}

	errs := Validate(m)
	if len(errs) == 0 {
		t.Error("expected error for missing spec")
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
