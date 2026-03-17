package manifest

import (
	"bytes"
	"fmt"
	"text/template"

	"gopkg.in/yaml.v3"
)

// compositeRegistry holds discovered composite definitions keyed by their name (the custom kind).
type compositeRegistry map[string]*compositeDefinition

// compositeDefinition stores a parsed CompositeDefinition's template resources.
type compositeDefinition struct {
	name      string
	templates []Resource
}

// ExpandComposites processes a manifest in two passes:
//  1. Extract CompositeDefinition resources into a registry.
//  2. Expand composite instances into their constituent resources.
func ExpandComposites(m *Manifest) error {
	registry, remaining, err := extractDefinitions(m.Resources)
	if err != nil {
		return err
	}

	if len(registry) == 0 {
		return nil
	}

	expanded, err := expandInstances(remaining, registry)
	if err != nil {
		return err
	}

	m.Resources = expanded
	return nil
}

// extractDefinitions separates CompositeDefinition resources from regular resources.
func extractDefinitions(resources []Resource) (compositeRegistry, []Resource, error) {
	registry := make(compositeRegistry)
	var remaining []Resource

	for _, r := range resources {
		if r.Kind != "CompositeDefinition" {
			remaining = append(remaining, r)
			continue
		}

		def, err := parseDefinition(r)
		if err != nil {
			return nil, nil, err
		}
		registry[r.Name] = def
	}

	return registry, remaining, nil
}

// parseDefinition builds a compositeDefinition from a CompositeDefinition resource.
func parseDefinition(r Resource) (*compositeDefinition, error) {
	rawResources, ok := r.Spec["resources"].([]any)
	if !ok {
		return nil, fmt.Errorf("CompositeDefinition %s: spec.resources must be an array", r.Name)
	}

	def := &compositeDefinition{name: r.Name}
	for i, raw := range rawResources {
		resMap, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("CompositeDefinition %s: resources[%d] must be a map", r.Name, i)
		}

		kind, _ := resMap["kind"].(string)
		name, _ := resMap["name"].(string)
		spec, _ := resMap["spec"].(map[string]any)

		def.templates = append(def.templates, Resource{
			Kind: ResourceKind(kind),
			Name: name,
			Spec: spec,
		})
	}

	return def, nil
}

// expandInstances replaces composite instances with their expanded resources.
func expandInstances(resources []Resource, registry compositeRegistry) ([]Resource, error) {
	var expanded []Resource

	for _, r := range resources {
		def, isComposite := registry[string(r.Kind)]
		if !isComposite {
			expanded = append(expanded, r)
			continue
		}

		ctx := buildContext(r)
		for _, tmpl := range def.templates {
			res, err := expandResource(tmpl, ctx)
			if err != nil {
				return nil, fmt.Errorf("expand %s/%s: %w", def.name, r.Name, err)
			}
			if len(r.Labels) > 0 {
				res.Labels = r.Labels
			}
			expanded = append(expanded, res)
		}
	}

	return expanded, nil
}

// buildContext creates the template context from a composite instance.
func buildContext(r Resource) map[string]any {
	ctx := make(map[string]any, len(r.Spec)+1)
	ctx["name"] = r.Name
	for k, v := range r.Spec {
		ctx[k] = v
	}
	return ctx
}

// expandResource applies template substitution to a resource's name and spec.
func expandResource(tmpl Resource, ctx map[string]any) (Resource, error) {
	name, err := renderTemplate(tmpl.Name, ctx)
	if err != nil {
		return Resource{}, fmt.Errorf("expand name: %w", err)
	}

	specYAML, err := yaml.Marshal(tmpl.Spec)
	if err != nil {
		return Resource{}, fmt.Errorf("marshal spec: %w", err)
	}

	expandedYAML, err := renderTemplate(string(specYAML), ctx)
	if err != nil {
		return Resource{}, fmt.Errorf("expand spec: %w", err)
	}

	var spec map[string]any
	if err := yaml.Unmarshal([]byte(expandedYAML), &spec); err != nil {
		return Resource{}, fmt.Errorf("unmarshal expanded spec: %w", err)
	}

	return Resource{Kind: tmpl.Kind, Name: name, Spec: spec}, nil
}

// renderTemplate applies Go text/template substitution.
func renderTemplate(text string, ctx map[string]any) (string, error) {
	t, err := template.New("").Parse(text)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
}
