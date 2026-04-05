package goclaw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// observeAgent fetches an agent by agentKey from GoClaw.
func (p *Provider) observeAgent(ctx context.Context, key string) (map[string]any, error) {
	data, err := p.http.Get(ctx, "/v1/agents")
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
		if strVal(a, "agent_key") == key && p.matchesTenant(ctx, a) {
			return translateResult(stripInternal(a)), nil
		}
	}
	return nil, nil
}

// createAgent creates a new agent in GoClaw.
// If contextFiles are present in the spec, they are synced via the import API
// after the agent is created (the main POST endpoint ignores context_files).
func (p *Provider) createAgent(ctx context.Context, key string, spec map[string]any) error {
	contextFiles, _ := spec["contextFiles"].([]any)

	body := translateSpec(spec)
	body["agent_key"] = key
	delete(body, "context_files") // not processed by create endpoint

	_, err := p.http.Post(ctx, "/v1/agents", body)
	if err != nil {
		return fmt.Errorf("create agent %s: %w", key, err)
	}

	if len(contextFiles) > 0 {
		if err := p.syncAgentContextFiles(ctx, key, contextFiles); err != nil {
			p.logger.Warn("sync context files after create",
				"agent", key, "error", err)
		}
	}
	return nil
}

// updateAgent updates an existing agent in GoClaw.
// If contextFiles are present in the spec, they are synced via the import API
// after the agent is updated (the PUT endpoint filters out context_files).
func (p *Provider) updateAgent(ctx context.Context, key string, spec map[string]any) error {
	contextFiles, _ := spec["contextFiles"].([]any)

	current, err := p.observeAgent(ctx, key)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("agent %s not found for update", key)
	}

	// current is already camelCase from observeAgent; extract id before translation
	id, ok := current["id"].(string)
	if !ok {
		return fmt.Errorf("agent %s: missing id", key)
	}

	body := translateSpec(spec)
	delete(body, "context_files") // not processed by update endpoint
	_, err = p.http.Put(ctx, "/v1/agents/"+id, body)
	if errors.Is(err, ErrNotFound) {
		return fmt.Errorf("agent %s (id=%s) not found: %w", key, id, err)
	}
	if err != nil {
		return err
	}

	if len(contextFiles) > 0 {
		if err := p.syncAgentContextFiles(ctx, key, contextFiles); err != nil {
			p.logger.Warn("sync context files after update",
				"agent", key, "error", err)
		}
	}
	return nil
}

// syncAgentContextFiles resolves the agent UUID and syncs context files via import.
func (p *Provider) syncAgentContextFiles(ctx context.Context, key string, files []any) error {
	current, err := p.observeAgent(ctx, key)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("agent %s not found", key)
	}
	id, ok := current["id"].(string)
	if !ok {
		return fmt.Errorf("agent %s: missing id", key)
	}
	return p.syncContextFiles(ctx, id, files)
}
