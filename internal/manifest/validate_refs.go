package manifest

import "fmt"

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
