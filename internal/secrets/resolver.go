// Package secrets resolves secret references in manifest values.
// Supports: ${ENV_VAR}, file:///path, SOPS-encrypted values.
package secrets

import (
	"fmt"
	"os"
	"path/filepath"
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
// Rejects paths containing ".." traversal sequences to prevent reading arbitrary files.
func ResolveFileRef(s string) (string, error) {
	path, ok := strings.CutPrefix(s, "file://")
	if !ok {
		return s, nil
	}
	cleaned := filepath.Clean(path)
	if strings.Contains(cleaned, "..") {
		return "", fmt.Errorf("secret file path %q contains path traversal", path)
	}
	data, err := os.ReadFile(cleaned)
	if err != nil {
		return "", fmt.Errorf("read secret file %s: %w", cleaned, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// Resolve applies all secret resolution strategies to a string value.
func Resolve(s string) (string, error) {
	// file:// takes precedence (entire value is a file ref)
	if _, ok := strings.CutPrefix(s, "file://"); ok {
		return ResolveFileRef(s)
	}
	// env var substitution
	return ResolveEnvVars(s)
}
