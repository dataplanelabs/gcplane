package reconciler

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
)

// WriteOnlyHashField is the spec field name used to store the content hash.
const WriteOnlyHashField = "writeOnlyHash"

// HashWriteOnlyFields computes a SHA-256 hash of write-only field values from a spec.
// Fields listed in ignoreFields are excluded from the hash.
// Returns empty string if no write-only fields are present or all are ignored.
func HashWriteOnlyFields(spec map[string]any, writeOnlyFields []string, ignoreFields []string) string {
	ignoreSet := make(map[string]bool, len(ignoreFields))
	for _, f := range ignoreFields {
		ignoreSet[f] = true
	}

	// Collect write-only field values in sorted order for deterministic hashing
	type kv struct {
		Key string
		Val any
	}
	var pairs []kv
	for _, field := range writeOnlyFields {
		if ignoreSet[field] {
			continue
		}
		val, ok := spec[field]
		if !ok {
			continue
		}
		pairs = append(pairs, kv{Key: field, Val: val})
	}

	if len(pairs) == 0 {
		return ""
	}

	sort.Slice(pairs, func(i, j int) bool { return pairs[i].Key < pairs[j].Key })

	// Build a deterministic map for JSON serialization
	ordered := make([]any, 0, len(pairs)*2)
	for _, p := range pairs {
		ordered = append(ordered, p.Key, p.Val)
	}

	data, err := json.Marshal(ordered)
	if err != nil {
		return ""
	}

	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}
