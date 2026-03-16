// Package keyconv provides bidirectional key conversion between camelCase and snake_case
// for map[string]any spec maps. Used by providers to translate between gcplane's
// canonical camelCase manifest format and backend-specific wire formats.
package keyconv

import (
	"strings"
	"unicode"
)

// CamelToSnake converts all keys in a map from camelCase to snake_case recursively.
func CamelToSnake(m map[string]any) map[string]any {
	return transformKeys(m, camelToSnake)
}

// SnakeToCamel converts all keys in a map from snake_case to camelCase recursively.
func SnakeToCamel(m map[string]any) map[string]any {
	return transformKeys(m, snakeToCamel)
}

// transformKeys recursively applies a key transformation function to all map keys.
func transformKeys(m map[string]any, fn func(string) string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[fn(k)] = transformValue(v, fn)
	}
	return out
}

// transformValue applies key transformation to nested maps and slices.
func transformValue(v any, fn func(string) string) any {
	switch val := v.(type) {
	case map[string]any:
		return transformKeys(val, fn)
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = transformValue(item, fn)
		}
		return result
	default:
		return v
	}
}

// camelToSnake converts a single camelCase key to snake_case.
// Examples: "displayName" → "display_name", "apiKey" → "api_key"
func camelToSnake(s string) string {
	if s == "" {
		return s
	}

	var b strings.Builder
	b.Grow(len(s) + 4) // pre-allocate for underscores

	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// snakeToCamel converts a single snake_case key to camelCase.
// Examples: "display_name" → "displayName", "api_key" → "apiKey"
func snakeToCamel(s string) string {
	if s == "" || !strings.Contains(s, "_") {
		return s
	}

	parts := strings.Split(s, "_")
	var b strings.Builder
	b.Grow(len(s))

	for i, part := range parts {
		if part == "" {
			continue
		}
		if i == 0 {
			b.WriteString(part)
		} else {
			// Capitalize first letter of subsequent parts
			runes := []rune(part)
			runes[0] = unicode.ToUpper(runes[0])
			b.WriteString(string(runes))
		}
	}
	return b.String()
}
