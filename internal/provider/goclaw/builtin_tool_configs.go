package goclaw

import (
	"context"
	"encoding/json"
	"fmt"
)

// observeBuiltinToolConfig checks if a builtin tool has a tenant-level config override.
// Key is the tool name (e.g., "exec", "web_fetch").
func (p *Provider) observeBuiltinToolConfig(key string) (map[string]any, error) {
	data, err := p.http.Get(context.Background(), "/v1/builtin-tools")
	if err != nil {
		return nil, fmt.Errorf("list builtin tools: %w", err)
	}

	var resp struct {
		Tools []map[string]any `json:"tools"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse builtin tools response: %w", err)
	}

	for _, t := range resp.Tools {
		if strVal(t, "name") == key {
			// Check if there's a tenant config override
			if tc, ok := t["tenant_config"].(map[string]any); ok {
				return translateResult(tc), nil
			}
			// Tool exists but no tenant config — return enabled state from tool itself
			result := map[string]any{
				"enabled": t["enabled"],
			}
			return translateResult(result), nil
		}
	}
	return nil, nil
}

// createBuiltinToolConfig sets a per-tenant config for a builtin tool.
func (p *Provider) createBuiltinToolConfig(key string, spec map[string]any) error {
	body := translateSpec(spec)
	path := fmt.Sprintf("/v1/builtin-tools/%s/tenant-config", key)
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
	path := fmt.Sprintf("/v1/builtin-tools/%s/tenant-config", key)
	return p.http.Delete(context.Background(), path)
}
