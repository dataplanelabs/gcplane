package goclaw

import (
	"context"
	"encoding/json"
	"fmt"
)

// observeSkillConfig checks if a skill has a tenant-level config override.
// Key is the skill slug.
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
		if strVal(s, "slug") == key {
			if tc, ok := s["tenant_config"].(map[string]any); ok {
				return translateResult(tc), nil
			}
			// Skill exists but no tenant config — return current enabled state
			result := map[string]any{
				"enabled": s["enabled"],
			}
			return translateResult(result), nil
		}
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
