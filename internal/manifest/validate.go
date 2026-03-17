package manifest

import (
	"fmt"
	"regexp"
)

var keyRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// validKinds is the set of supported resource kinds.
var validKinds = map[ResourceKind]bool{
	KindProvider:  true,
	KindAgent:     true,
	KindChannel:   true,
	KindCronJob:   true,
	KindMCPServer: true,
	KindSkill:     true,
	KindTool:      true,
	KindAgentTeam:      true,
	KindTTSConfig: true,
}

// Validate checks the manifest for structural errors.
func Validate(m *Manifest) []error {
	var errs []error

	if m.APIVersion != "gcplane.io/v1" {
		errs = append(errs, fmt.Errorf("unsupported apiVersion %q, expected gcplane.io/v1", m.APIVersion))
	}

	if m.Kind != "Manifest" {
		errs = append(errs, fmt.Errorf("unsupported kind %q, expected Manifest", m.Kind))
	}

	seen := make(map[string]bool)
	for i, r := range m.Resources {
		prefix := fmt.Sprintf("resources[%d]", i)

		if !validKinds[r.Kind] {
			errs = append(errs, fmt.Errorf("%s: unknown kind %q", prefix, r.Kind))
		}

		if r.Name == "" {
			errs = append(errs, fmt.Errorf("%s: name is required", prefix))
		} else if !keyRe.MatchString(r.Name) {
			errs = append(errs, fmt.Errorf("%s: name %q must be kebab-case (a-z0-9, hyphens)", prefix, r.Name))
		}

		uid := fmt.Sprintf("%s/%s", r.Kind, r.Name)
		if seen[uid] {
			errs = append(errs, fmt.Errorf("%s: duplicate resource %s", prefix, uid))
		}
		seen[uid] = true

		if r.Spec == nil {
			errs = append(errs, fmt.Errorf("%s: spec is required for %s", prefix, uid))
		}
	}

	// Cross-resource reference validation
	errs = append(errs, validateReferences(m)...)

	return errs
}
