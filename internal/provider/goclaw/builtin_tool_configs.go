package goclaw

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// toolName converts kebab-case manifest name to snake_case GoClaw tool name.
func toolName(key string) string {
	return strings.ReplaceAll(key, "-", "_")
}

// observeBuiltinToolConfig checks if a builtin tool has a tenant-level config override.
// Key is kebab-case manifest name (e.g., "exec", "web-fetch") — converted to snake_case for API.
func (p *Provider) observeBuiltinToolConfig(key string) (map[string]any, error) {
	data, err := p.http.Get(context.Background(), "/v1/tools/builtin")
	if err != nil {
		return nil, fmt.Errorf("list builtin tools: %w", err)
	}

	var resp struct {
		Tools []map[string]any `json:"tools"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse builtin tools response: %w", err)
	}

	apiName := toolName(key)
	for _, t := range resp.Tools {
		if strVal(t, "name") == apiName {
			// GoClaw list response includes "tenant_enabled" when tenant-scoped.
			// If present, it's the per-tenant override; otherwise use global "enabled".
			if te, ok := t["tenant_enabled"]; ok {
				return map[string]any{"enabled": te}, nil
			}
			return map[string]any{"enabled": t["enabled"]}, nil
		}
	}
	return nil, nil
}

// createBuiltinToolConfig sets a per-tenant config for a builtin tool.
func (p *Provider) createBuiltinToolConfig(key string, spec map[string]any) error {
	body := translateSpec(spec)
	path := fmt.Sprintf("/v1/tools/builtin/%s/tenant-config", toolName(key))
	_, err := p.http.Put(context.Background(), path, body)
	if err != nil {
		return fmt.Errorf("set builtin tool config %s: %w", key, err)
	}
	return nil
}

// updateBuiltinToolConfig is the same as create — PUT is upsert.
func (p *Provider) updateBuiltinToolConfig(key string, spec map[string]any) error {
	return p.createBuiltinToolConfig(key, spec)
}

// deleteBuiltinToolConfig removes a per-tenant config override for a builtin tool.
func (p *Provider) deleteBuiltinToolConfig(key string) error {
	path := fmt.Sprintf("/v1/tools/builtin/%s/tenant-config", toolName(key))
	return p.http.Delete(context.Background(), path)
}
