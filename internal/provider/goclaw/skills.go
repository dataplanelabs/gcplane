package goclaw

import (
	"context"
	"encoding/json"
	"fmt"
)

// observeSkill fetches a skill by key from GoClaw.
func (p *Provider) observeSkill(key string) (map[string]any, error) {
	data, err := p.http.Get(context.Background(), "/v1/skills")
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
		if strVal(s, "key") == key {
			return translateResult(stripInternal(s)), nil
		}
	}
	return nil, nil
}

// updateSkill updates an existing skill in GoClaw.
// Skills are auto-discovered; only update is supported.
func (p *Provider) updateSkill(key string, spec map[string]any) error {
	current, err := p.observeSkill(key)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("skill %s not found for update", key)
	}

	id, ok := current["id"].(string)
	if !ok {
		return fmt.Errorf("skill %s: missing id", key)
	}

	body := translateSpec(spec)
	_, err = p.http.Put(context.Background(), "/v1/skills/"+id, body)
	return err
}
