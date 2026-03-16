package goclaw

import "github.com/dataplanelabs/gcplane/internal/keyconv"

// translateSpec converts manifest camelCase keys to GoClaw snake_case for API calls.
func translateSpec(spec map[string]any) map[string]any {
	return keyconv.CamelToSnake(spec)
}

// translateResult converts GoClaw snake_case keys to manifest camelCase for comparison.
func translateResult(result map[string]any) map[string]any {
	return keyconv.SnakeToCamel(result)
}

// strVal safely extracts a string value from a map.
func strVal(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// copyMap creates a shallow copy of a map.
func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
