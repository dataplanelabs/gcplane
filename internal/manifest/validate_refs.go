package manifest

import (
	"fmt"
	"strings"
)

// splitAgentLinkName mirrors the provider-side parser for composite AgentLink
// names. Kept here so the manifest validator can reject malformed names without
// importing the provider package.
func splitAgentLinkName(name string) (string, string, error) {
	parts := strings.Split(name, "--")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid AgentLink name %q: expected format sourceAgent--targetAgent", name)
	}
	return parts[0], parts[1], nil
}

// validateReferences checks cross-resource references in the manifest.
// Returns all broken references as errors (not fail-on-first).
func validateReferences(m *Manifest) []error {
	// Build index: kind → set of declared names
	index := make(map[ResourceKind]map[string]bool)
	for _, r := range m.Resources {
		if index[r.Kind] == nil {
			index[r.Kind] = make(map[string]bool)
		}
		index[r.Kind][r.Name] = true
	}

	var errs []error
	for i, r := range m.Resources {
		prefix := fmt.Sprintf("resources[%d] %s/%s", i, r.Kind, r.Name)
		switch r.Kind {
		case KindAgent:
			// spec.provider must reference a Provider
			if ref := specStr(r.Spec, "provider"); ref != "" {
				if !index[KindProvider][ref] {
					errs = append(errs, fmt.Errorf("%s: references Provider %q which is not declared", prefix, ref))
				}
			}
		case KindChannel:
			// spec.agentKey must reference an Agent
			if ref := specStr(r.Spec, "agentKey"); ref != "" {
				if !index[KindAgent][ref] {
					errs = append(errs, fmt.Errorf("%s: references Agent %q which is not declared", prefix, ref))
				}
			}
		case KindCronJob:
			// spec.agentKey must reference an Agent
			if ref := specStr(r.Spec, "agentKey"); ref != "" {
				if !index[KindAgent][ref] {
					errs = append(errs, fmt.Errorf("%s: references Agent %q which is not declared", prefix, ref))
				}
			}
		case KindMCPServer:
			// spec.grants.agents[] must reference Agents
			if grants, ok := r.Spec["grants"].(map[string]any); ok {
				for _, agent := range specStrSlice(grants, "agents") {
					if !index[KindAgent][agent] {
						errs = append(errs, fmt.Errorf("%s: grants references Agent %q which is not declared", prefix, agent))
					}
				}
			}
		case KindSecureCLIGrant:
			// spec.agentKey must reference an Agent
			if ref := specStr(r.Spec, "agentKey"); ref != "" {
				if !index[KindAgent][ref] {
					errs = append(errs, fmt.Errorf("%s: references Agent %q which is not declared", prefix, ref))
				}
			}
			// spec.binaryName must reference a SecureCLI
			if ref := specStr(r.Spec, "binaryName"); ref != "" {
				if !index[KindSecureCLI][ref] {
					errs = append(errs, fmt.Errorf("%s: references SecureCLI %q which is not declared", prefix, ref))
				}
			}
		case KindAgentTeam:
			// spec.lead must reference an Agent
			if ref := specStr(r.Spec, "lead"); ref != "" {
				if !index[KindAgent][ref] {
					errs = append(errs, fmt.Errorf("%s: lead references Agent %q which is not declared", prefix, ref))
				}
			}
			// spec.members[] must reference Agents
			for _, member := range specStrSlice(r.Spec, "members") {
				if !index[KindAgent][member] {
					errs = append(errs, fmt.Errorf("%s: member references Agent %q which is not declared", prefix, member))
				}
			}
		case KindAgentLink:
			// Composite name "sourceAgent--targetAgent" is the canonical identity.
			// spec.sourceAgent / spec.targetAgent must agree with the name halves
			// (when provided) to prevent silent divergence between manifest intent
			// and the resolved API call.
			nameSrc, nameTgt, parseErr := splitAgentLinkName(r.Name)
			if parseErr != nil {
				errs = append(errs, fmt.Errorf("%s: %v", prefix, parseErr))
				continue
			}
			specSrc := specStr(r.Spec, "sourceAgent")
			specTgt := specStr(r.Spec, "targetAgent")
			if specSrc != "" && specSrc != nameSrc {
				errs = append(errs, fmt.Errorf("%s: spec.sourceAgent %q must match name prefix %q", prefix, specSrc, nameSrc))
			}
			if specTgt != "" && specTgt != nameTgt {
				errs = append(errs, fmt.Errorf("%s: spec.targetAgent %q must match name suffix %q", prefix, specTgt, nameTgt))
			}
			// Both halves must reference declared Agents.
			if !index[KindAgent][nameSrc] {
				errs = append(errs, fmt.Errorf("%s: sourceAgent references Agent %q which is not declared", prefix, nameSrc))
			}
			if !index[KindAgent][nameTgt] {
				errs = append(errs, fmt.Errorf("%s: targetAgent references Agent %q which is not declared", prefix, nameTgt))
			}
		}
	}
	return errs
}

// specStr extracts a string field from a spec map.
func specStr(spec map[string]any, key string) string {
	if v, ok := spec[key].(string); ok {
		return v
	}
	return ""
}

// specStrSlice extracts a string slice from a spec map.
// Handles []any with string elements (YAML unmarshals arrays as []any).
func specStrSlice(spec map[string]any, key string) []string {
	arr, ok := spec[key].([]any)
	if !ok {
		return nil
	}
	var result []string
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
