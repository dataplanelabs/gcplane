package goclaw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// observeChannelInstance fetches a channel instance by name from GoClaw.
func (p *Provider) observeChannelInstance(key string) (map[string]any, error) {
	data, err := p.http.Get(context.Background(), "/v1/channels/instances")
	if err != nil {
		return nil, fmt.Errorf("list channel instances: %w", err)
	}

	var resp struct {
		Instances []map[string]any `json:"instances"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse channel instances response: %w", err)
	}

	for _, inst := range resp.Instances {
		if strVal(inst, "name") == key {
			return translateResult(inst), nil
		}
	}
	return nil, nil
}

// createChannelInstance creates a new channel instance in GoClaw.
// Resolves agentKey → agent_id UUID before sending.
func (p *Provider) createChannelInstance(key string, spec map[string]any) error {
	body := translateSpec(spec)
	body["name"] = key

	// Resolve agent_key → agent_id (GoClaw expects UUID)
	if agentKey, ok := body["agent_key"].(string); ok {
		agentID, err := p.resolveAgentID(agentKey)
		if err != nil {
			return fmt.Errorf("channel %s: %w", key, err)
		}
		body["agent_id"] = agentID
		delete(body, "agent_key")
	}

	_, err := p.http.Post(context.Background(), "/v1/channels/instances", body)
	if err != nil {
		return fmt.Errorf("create channel instance %s: %w", key, err)
	}
	return nil
}

// updateChannelInstance updates an existing channel instance in GoClaw.
func (p *Provider) updateChannelInstance(key string, spec map[string]any) error {
	current, err := p.observeChannelInstance(key)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("channel instance %s not found for update", key)
	}

	id, ok := current["id"].(string)
	if !ok {
		return fmt.Errorf("channel instance %s: missing id", key)
	}

	body := translateSpec(spec)
	_, err = p.http.Put(context.Background(), "/v1/channels/instances/"+id, body)
	if errors.Is(err, ErrNotFound) {
		return fmt.Errorf("channel instance %s (id=%s) not found: %w", key, id, err)
	}
	return err
}
