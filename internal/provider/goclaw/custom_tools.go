package goclaw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// observeCustomTool fetches a custom tool by name from GoClaw.
func (p *Provider) observeCustomTool(key string) (map[string]any, error) {
	data, err := p.http.Get(context.Background(), "/v1/tools/custom")
	if err != nil {
		return nil, fmt.Errorf("list custom tools: %w", err)
	}

	var resp struct {
		Tools []map[string]any `json:"tools"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse custom tools response: %w", err)
	}

	for _, t := range resp.Tools {
		if strVal(t, "name") == key {
			return translateResult(t), nil
		}
	}
	return nil, nil
}

// createCustomTool creates a new custom tool in GoClaw.
func (p *Provider) createCustomTool(key string, spec map[string]any) error {
	body := translateSpec(spec)
	body["name"] = key

	_, err := p.http.Post(context.Background(), "/v1/tools/custom", body)
	if err != nil {
		return fmt.Errorf("create custom tool %s: %w", key, err)
	}
	return nil
}

// updateCustomTool updates an existing custom tool in GoClaw.
func (p *Provider) updateCustomTool(key string, spec map[string]any) error {
	current, err := p.observeCustomTool(key)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("custom tool %s not found for update", key)
	}

	id, ok := current["id"].(string)
	if !ok {
		return fmt.Errorf("custom tool %s: missing id", key)
	}

	body := translateSpec(spec)
	_, err = p.http.Put(context.Background(), "/v1/tools/custom/"+id, body)
	if errors.Is(err, ErrNotFound) {
		return fmt.Errorf("custom tool %s (id=%s) not found: %w", key, id, err)
	}
	return err
}
