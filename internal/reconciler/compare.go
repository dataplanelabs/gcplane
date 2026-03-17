package reconciler

import (
	"fmt"
	"reflect"
)

// CompareSpec deep-compares desired vs actual spec maps.
// Returns changed fields only.
func CompareSpec(desired, actual map[string]any) map[string]FieldDiff {
	return CompareSpecExcluding(desired, actual, nil)
}

// CompareSpecExcluding deep-compares desired vs actual spec maps,
// skipping top-level keys listed in exclude (write-only fields not returned by the API).
func CompareSpecExcluding(desired, actual map[string]any, exclude []string) map[string]FieldDiff {
	excludeSet := make(map[string]bool, len(exclude))
	for _, f := range exclude {
		excludeSet[f] = true
	}
	diffs := make(map[string]FieldDiff)
	compareRecursiveExcluding("", desired, actual, diffs, excludeSet)
	return diffs
}

func compareRecursiveExcluding(prefix string, desired, actual map[string]any, diffs map[string]FieldDiff, excludeSet map[string]bool) {
	for key, dVal := range desired {
		// Skip write-only fields at top level
		if prefix == "" && excludeSet[key] {
			continue
		}

		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		aVal, exists := actual[key]

		if !exists {
			diffs[path] = FieldDiff{Old: nil, New: dVal}
			continue
		}

		// Both are maps — recurse
		dMap, dIsMap := toMap(dVal)
		aMap, aIsMap := toMap(aVal)
		if dIsMap && aIsMap {
			compareRecursiveExcluding(path, dMap, aMap, diffs, excludeSet)
			continue
		}

		// If the API masks a secret field with "***", skip — we cannot compare
		if aVal == "***" {
			continue
		}

		// Compare values
		if !valuesEqual(dVal, aVal) {
			diffs[path] = FieldDiff{Old: aVal, New: dVal}
		}
	}
}

// toMap converts a value to map[string]any if possible.
func toMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[fmt.Sprintf("%v", k)] = val
		}
		return out, true
	default:
		return nil, false
	}
}

// valuesEqual compares two values, handling numeric type mismatches from JSON.
func valuesEqual(a, b any) bool {
	// Handle numeric comparisons (JSON unmarshals to float64, YAML may give int)
	aNum, aIsNum := toFloat64(a)
	bNum, bIsNum := toFloat64(b)
	if aIsNum && bIsNum {
		return aNum == bNum
	}

	return reflect.DeepEqual(a, b)
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}
