package manifest

import "strings"

// FilterByLabels returns resources matching all specified label key=value pairs.
// Empty selector returns all resources.
func FilterByLabels(resources []Resource, selector map[string]string) []Resource {
	if len(selector) == 0 {
		return resources
	}
	var filtered []Resource
	for _, r := range resources {
		if matchesLabels(r.Labels, selector) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// matchesLabels returns true if resource labels contain all selector pairs.
func matchesLabels(labels, selector map[string]string) bool {
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

// ParseLabelSelector parses "key=value,key2=value2" into a map.
func ParseLabelSelector(s string) map[string]string {
	if s == "" {
		return nil
	}
	result := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result
}
