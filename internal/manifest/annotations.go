package manifest

import "strings"

// Annotation keys for resource-level reconciliation control.
const (
	// AnnotationIgnoreWriteOnly disables write-only hash tracking for a resource.
	// Value: "true" to skip hash computation entirely.
	AnnotationIgnoreWriteOnly = "gcplane.io/ignore-write-only"

	// AnnotationIgnoreFields excludes specific fields from write-only hash computation.
	// Value: comma-separated field names (e.g., "apiKey,systemPrompt").
	AnnotationIgnoreFields = "gcplane.io/ignore-fields"

	// AnnotationSyncPolicy controls reconciliation behavior for a resource.
	// Value: "Ignore" to skip reconciliation entirely.
	AnnotationSyncPolicy = "gcplane.io/sync-policy"

	// SyncPolicyIgnore skips reconciliation for a resource.
	SyncPolicyIgnore = "Ignore"
)

// ParseIgnoreFields splits a comma-separated annotation value into field names.
func ParseIgnoreFields(annotations map[string]string) []string {
	val, ok := annotations[AnnotationIgnoreFields]
	if !ok || val == "" {
		return nil
	}
	parts := strings.Split(val, ",")
	fields := make([]string, 0, len(parts))
	for _, p := range parts {
		if f := strings.TrimSpace(p); f != "" {
			fields = append(fields, f)
		}
	}
	return fields
}
