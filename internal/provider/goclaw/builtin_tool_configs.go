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
func (p *Provider) observeBuiltinToolConfig(ctx context.Context, key string) (map[string]any, error) {
	data, err := p.http.Get(ctx, "/v1/tools/builtin")
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
			// GoClaw list response includes "tenant_enabled" only when a per-tenant
			// config override exists. If absent, the tool uses the global "enabled"
			// and we return nil so the reconciler creates the tenant config.
			if te, ok := t["tenant_enabled"]; ok {
				return map[string]any{"enabled": te}, nil
			}
			return nil, nil
		}
	}
	return nil, nil
}

// createBuiltinToolConfig sets a per-tenant config for a builtin tool.
// When settings are provided (e.g., provider chain), also updates the global
// tool definition since the tenant-config endpoint only persists enabled state.
func (p *Provider) createBuiltinToolConfig(ctx context.Context, key string, spec map[string]any) error {
	body := translateSpec(spec)

	// Update tenant-level enabled state
	path := fmt.Sprintf("/v1/tools/builtin/%s/tenant-config", toolName(key))
	_, err := p.http.Put(ctx, path, body)
	if err != nil {
		return fmt.Errorf("set builtin tool config %s: %w", key, err)
	}

	// If settings are provided, also update the global tool definition
	// because the tenant-config endpoint only persists the enabled flag.
	if _, hasSettings := body["settings"]; hasSettings {
		globalPath := fmt.Sprintf("/v1/tools/builtin/%s", toolName(key))
		settingsOnly := map[string]any{"settings": body["settings"]}
		if _, err := p.http.Put(ctx, globalPath, settingsOnly); err != nil {
			return fmt.Errorf("update global tool settings %s: %w", key, err)
		}
	}

	return nil
}

// updateBuiltinToolConfig is the same as create — PUT is upsert.
func (p *Provider) updateBuiltinToolConfig(ctx context.Context, key string, spec map[string]any) error {
	return p.createBuiltinToolConfig(ctx, key, spec)
}

// deleteBuiltinToolConfig removes a per-tenant config override for a builtin tool.
func (p *Provider) deleteBuiltinToolConfig(ctx context.Context, key string) error {
	path := fmt.Sprintf("/v1/tools/builtin/%s/tenant-config", toolName(key))
	return p.http.Delete(ctx, path)
}
