package manifest

import (
	"fmt"
	"regexp"

	"github.com/dataplanelabs/gcplane/internal/skillpkg"
)

var keyRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// sysConfigKeyRe matches GoClaw system config keys (alphanumeric, dots, underscores, hyphens).
var sysConfigKeyRe = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,100}$`)

// validKinds is the set of supported resource kinds.
var validKinds = map[ResourceKind]bool{
	KindTenant:            true,
	KindProvider:          true,
	KindAgent:             true,
	KindChannel:           true,
	KindCronJob:           true,
	KindMCPServer:         true,
	KindSkill:             true,
	KindAgentTeam:         true,
	KindBuiltinToolConfig: true,
	KindSkillConfig:       true,
	KindSystemConfig:      true,
	KindMCPCredentials:    true,
	KindSecureCLI:         true,
	KindSecureCLIGrant:    true,
	KindAgentLink:         true,
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
		} else if r.Kind == KindSystemConfig {
			if !sysConfigKeyRe.MatchString(r.Name) {
				errs = append(errs, fmt.Errorf("%s: name %q must match [a-zA-Z0-9._-]{1,100}", prefix, r.Name))
			}
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

		if r.Kind == KindSkill {
			errs = append(errs, validateSkillSpec(prefix, r.Spec)...)
		}
	}

	// Cross-resource reference validation
	errs = append(errs, validateReferences(m)...)

	return errs
}

// Skill enum values mirror goclaw's DB CHECK constraints (see goclaw
// internal/store/sqlitestore/schema.sql `skills` table). Keep in sync.
var (
	skillVisibilityValues = map[string]bool{"private": true, "internal": true, "public": true, "tenant": true}
	skillStatusValues     = map[string]bool{"active": true, "archived": true, "deleted": true, "draft": true}
)

func validateSkillSpec(prefix string, spec map[string]any) []error {
	if spec == nil {
		return nil
	}
	var errs []error
	if v, ok := spec["visibility"].(string); ok && v != "" {
		if !skillVisibilityValues[v] {
			errs = append(errs, fmt.Errorf("%s: spec.visibility %q must be one of [private internal public tenant]", prefix, v))
		}
	}
	if s, ok := spec["status"].(string); ok && s != "" {
		if !skillStatusValues[s] {
			errs = append(errs, fmt.Errorf("%s: spec.status %q must be one of [active archived deleted draft]", prefix, s))
		}
	}
	if tags, ok := spec["tags"]; ok && tags != nil {
		list, isList := tags.([]any)
		if !isList {
			errs = append(errs, fmt.Errorf("%s: spec.tags must be a list of strings", prefix))
		} else {
			seen := make(map[string]bool, len(list))
			for i, t := range list {
				s, ok := t.(string)
				if !ok {
					errs = append(errs, fmt.Errorf("%s: spec.tags[%d] must be a string", prefix, i))
					continue
				}
				if seen[s] {
					errs = append(errs, fmt.Errorf("%s: spec.tags duplicate %q", prefix, s))
				}
				seen[s] = true
			}
		}
	}
	if src, ok := spec["sourceDir"].(string); ok && src != "" {
		if _, err := skillpkg.PackDir(src); err != nil {
			errs = append(errs, fmt.Errorf("%s: spec.sourceDir %q: %w", prefix, src, err))
		}
	}
	return errs
}
