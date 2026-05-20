package goclaw

import (
	"context"
	"encoding/json"
	"fmt"
)

// observeSkillConfig checks if a skill has a tenant-level config override.
// Key is the skill slug.
//
// goclaw's GET /v1/skills returns a flat `tenant_enabled` field per skill when
// the caller is tenant-scoped. The field is a JSON-null when no override exists,
// and a bool (true/false) when an override is present. We exploit the nil-ness
// of the unmarshalled pointer to distinguish "no override" from "explicit value".
func (p *Provider) observeSkillConfig(ctx context.Context, key string) (map[string]any, error) {
	data, err := p.http.Get(ctx, "/v1/skills")
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}

	var resp struct {
		Skills []map[string]any `json:"skills"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse skills response: %w", err)
	}

	for _, s := range resp.Skills {
		if strVal(s, "slug") != key {
			continue
		}
		raw, present := s["tenant_enabled"]
		if !present || raw == nil {
			// Field absent or JSON null → no tenant override
			return nil, nil
		}
		enabled, ok := raw.(bool)
		if !ok {
			return nil, fmt.Errorf("skill %q: tenant_enabled has unexpected type %T", key, raw)
		}
		return translateResult(map[string]any{"enabled": enabled}), nil
	}
	return nil, nil
}

// resolveSkillID looks up a skill by slug and returns its UUID.
func (p *Provider) resolveSkillID(ctx context.Context, slug string) (string, error) {
	data, err := p.http.Get(ctx, "/v1/skills")
	if err != nil {
		return "", fmt.Errorf("list skills: %w", err)
	}

	var resp struct {
		Skills []map[string]any `json:"skills"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("parse skills response: %w", err)
	}

	for _, s := range resp.Skills {
		if strVal(s, "slug") == slug {
			if id := strVal(s, "id"); id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("skill %q not found", slug)
}

// createSkillConfig sets a per-tenant config for a skill.
func (p *Provider) createSkillConfig(ctx context.Context, key string, spec map[string]any) error {
	id, err := p.resolveSkillID(ctx, key)
	if err != nil {
		return err
	}

	body := translateSpec(spec)
	path := fmt.Sprintf("/v1/skills/%s/tenant-config", id)
	_, err = p.http.Put(ctx, path, body)
	if err != nil {
		return fmt.Errorf("set skill config %s: %w", key, err)
	}
	return nil
}

// updateSkillConfig is the same as create — PUT is upsert.
func (p *Provider) updateSkillConfig(ctx context.Context, key string, spec map[string]any) error {
	return p.createSkillConfig(ctx, key, spec)
}

// deleteSkillConfig removes a per-tenant config override for a skill.
func (p *Provider) deleteSkillConfig(ctx context.Context, key string) error {
	id, err := p.resolveSkillID(ctx, key)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/v1/skills/%s/tenant-config", id)
	return p.http.Delete(ctx, path)
}
