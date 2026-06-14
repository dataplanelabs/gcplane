package manifest

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"

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
	KindWorkstation:       true,
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
	// requires.cli: only `>=X.Y` shape supported in this release. Reject
	// npm-style (^X.Y, ~X.Y) and ranges (>=X.Y, <Z) at validate time so
	// the apply path doesn't have to handle unsupported syntax.
	if reqs, ok := spec["requires"].(map[string]any); ok {
		if cli, ok := reqs["cli"].(map[string]any); ok {
			for binary, raw := range cli {
				constraint, isStr := raw.(string)
				if !isStr {
					errs = append(errs, fmt.Errorf("%s: spec.requires.cli.%s must be a string", prefix, binary))
					continue
				}
				if err := validateCliConstraintShape(constraint); err != nil {
					errs = append(errs, fmt.Errorf("%s: spec.requires.cli.%s: %w", prefix, binary, err))
				}
			}
		}
	}
	return errs
}

// validateCliConstraintShape enforces the MVP constraint syntax for
// requires.cli: only `>=X.Y` is supported. npm-style `^` / `~`, ranges,
// and exact-pinning are rejected with an actionable error message so
// skill authors learn the supported shape early.
func validateCliConstraintShape(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("constraint must not be empty")
	}
	if strings.ContainsAny(s, "^~") {
		return fmt.Errorf("unsupported constraint shape %q; only >=X.Y supported in this release", s)
	}
	if strings.ContainsAny(s, ",") {
		return fmt.Errorf("unsupported constraint shape %q; only >=X.Y supported in this release", s)
	}
	if !strings.HasPrefix(s, ">=") {
		return fmt.Errorf("unsupported constraint shape %q; only >=X.Y supported in this release", s)
	}
	rest := strings.TrimSpace(strings.TrimPrefix(s, ">="))
	if rest == "" {
		return fmt.Errorf("missing version after >= in %q", s)
	}
	if !semver.IsValid(normalizeSemver(rest)) {
		return fmt.Errorf("invalid semver %q after >=", rest)
	}
	return nil
}

// normalizeSemver prepends 'v' to a bare semver string so golang.org/x/mod/semver
// can compare it (its API requires the v-prefix). Idempotent — strings that
// already start with 'v' are returned as-is.
func normalizeSemver(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if s[0] == 'v' || s[0] == 'V' {
		return s
	}
	return "v" + s
}

// CrossCheckRequiresCli compares spec.requires.cli constraints against the
// installed versions returned by the goclaw `/v1/system/cli-versions`
// endpoint. Called at apply time (createSkill / updateSkill) so the
// surface stays unit-testable without HTTP.
//
// Semantics:
//   - constraint shape errors → returned as error (would have been caught
//     by validateCliConstraintShape too, but defense-in-depth)
//   - binary not in installed map → returned as warning string, not error
//     (skill author may have asked for a future binary; warn don't block)
//   - installed version older than constraint → returned as error
//   - all checks pass → nil error, nil warnings
func CrossCheckRequiresCli(spec map[string]any, installed map[string]string) (warnings []string, err error) {
	if spec == nil {
		return nil, nil
	}
	reqs, ok := spec["requires"].(map[string]any)
	if !ok {
		return nil, nil
	}
	cli, ok := reqs["cli"].(map[string]any)
	if !ok {
		return nil, nil
	}
	for binary, raw := range cli {
		constraint, isStr := raw.(string)
		if !isStr {
			return warnings, fmt.Errorf("requires.cli.%s must be a string", binary)
		}
		if err := validateCliConstraintShape(constraint); err != nil {
			return warnings, fmt.Errorf("requires.cli.%s: %w", binary, err)
		}
		want := normalizeSemver(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(constraint), ">=")))
		got, ok := installed[binary]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("requires.cli.%s: binary not registered on server — skipping version check", binary))
			continue
		}
		if semver.Compare(normalizeSemver(got), want) < 0 {
			return warnings, fmt.Errorf("requires.cli.%s: installed %s does not satisfy %s", binary, got, constraint)
		}
	}
	return warnings, nil
}
