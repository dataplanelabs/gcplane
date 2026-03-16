package goclaw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// observeAgent fetches an agent by agentKey from GoClaw.
func (p *Provider) observeAgent(key string) (map[string]any, error) {
	data, err := p.http.Get(context.Background(), "/v1/agents")
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}

	var resp struct {
		Agents []map[string]any `json:"agents"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse agents response: %w", err)
	}

	for _, a := range resp.Agents {
		if strVal(a, "agentKey") == key {
			return a, nil
		}
	}
	return nil, nil
}

// createAgent creates a new agent in GoClaw.
func (p *Provider) createAgent(key string, spec map[string]any) error {
	body := copyMap(spec)
	body["agentKey"] = key

	_, err := p.http.Post(context.Background(), "/v1/agents", body)
	if err != nil {
		return fmt.Errorf("create agent %s: %w", key, err)
	}
	return nil
}

// updateAgent updates an existing agent in GoClaw.
func (p *Provider) updateAgent(key string, spec map[string]any) error {
	current, err := p.observeAgent(key)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("agent %s not found for update", key)
	}

	id, ok := current["id"].(string)
	if !ok {
		return fmt.Errorf("agent %s: missing id", key)
	}

	body := copyMap(spec)
	_, err = p.http.Put(context.Background(), "/v1/agents/"+id, body)
	if errors.Is(err, ErrNotFound) {
		return fmt.Errorf("agent %s (id=%s) not found: %w", key, id, err)
	}
	return err
}
