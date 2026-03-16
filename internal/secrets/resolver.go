// Package secrets resolves secret references in manifest values.
// Supports: ${ENV_VAR}, file:///path, SOPS-encrypted values.
package secrets

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var envVarRe = regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)\}`)

// ResolveEnvVars replaces all ${ENV_VAR} references in a string.
func ResolveEnvVars(s string) (string, error) {
	var missing []string

	result := envVarRe.ReplaceAllStringFunc(s, func(match string) string {
		key := envVarRe.FindStringSubmatch(match)[1]
		val, ok := os.LookupEnv(key)
		if !ok {
			missing = append(missing, key)
			return match
		}
		return val
	})

	if len(missing) > 0 {
		return result, fmt.Errorf("unresolved env vars: %s", strings.Join(missing, ", "))
	}
	return result, nil
}

// ResolveFileRef reads a file:// reference and returns its contents.
func ResolveFileRef(s string) (string, error) {
	if !strings.HasPrefix(s, "file://") {
		return s, nil
	}
	path := strings.TrimPrefix(s, "file://")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read secret file %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// Resolve applies all secret resolution strategies to a string value.
func Resolve(s string) (string, error) {
	// file:// takes precedence (entire value is a file ref)
	if strings.HasPrefix(s, "file://") {
		return ResolveFileRef(s)
	}
	// env var substitution
	return ResolveEnvVars(s)
}
