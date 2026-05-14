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
// When a tenant override exists, the list API returns "tenant_enabled" and optionally
// "tenant_settings" — both are surfaced in the observed state so drift detection works.
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
		if strVal(t, "name") != apiName {
			continue
		}
		// GoClaw list response includes "tenant_enabled" only when a per-tenant
		// config override exists. If absent, the tool uses the global defaults
		// and we return nil so the reconciler creates the tenant config.
		te, hasTenantEnabled := t["tenant_enabled"]
		if !hasTenantEnabled {
			return nil, nil
		}
		observed := map[string]any{"enabled": te}
		// "tenant_settings" is present when the tenant has a settings override.
		// Include it so drift detection can compare against the desired spec.
		if ts, ok := t["tenant_settings"]; ok && ts != nil {
			// tenant_settings arrives as map[string]any from JSON decode.
			observed["settings"] = ts
		}
		return observed, nil
	}
	return nil, nil
}

// createBuiltinToolConfig sets a per-tenant config for a builtin tool.
// Both enabled and settings are written to the tenant-config endpoint, which
// persists each field independently so either can be omitted without clearing
// the other. The previous approach of writing settings to the global PUT
// endpoint required master-scope access and applied changes to all tenants.
func (p *Provider) createBuiltinToolConfig(ctx context.Context, key string, spec map[string]any) error {
	body := translateSpec(spec)

	path := fmt.Sprintf("/v1/tools/builtin/%s/tenant-config", toolName(key))
	_, err := p.http.Put(ctx, path, body)
	if err != nil {
		return fmt.Errorf("set builtin tool config %s: %w", key, err)
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
