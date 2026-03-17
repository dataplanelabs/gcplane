package goclaw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// observeMCPServer fetches an MCP server by key from GoClaw.
func (p *Provider) observeMCPServer(key string) (map[string]any, error) {
	data, err := p.http.Get(context.Background(), "/v1/mcp/servers")
	if err != nil {
		return nil, fmt.Errorf("list mcp servers: %w", err)
	}

	var resp struct {
		Servers []map[string]any `json:"servers"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse mcp servers response: %w", err)
	}

	for _, s := range resp.Servers {
		if strVal(s, "name") == key {
			return translateResult(s), nil
		}
	}
	return nil, nil
}

// createMCPServer creates a new MCP server in GoClaw.
func (p *Provider) createMCPServer(key string, spec map[string]any) error {
	body := translateSpec(spec)
	body["name"] = key

	_, err := p.http.Post(context.Background(), "/v1/mcp/servers", body)
	if err != nil {
		return fmt.Errorf("create mcp server %s: %w", key, err)
	}
	return nil
}

// updateMCPServer updates an existing MCP server in GoClaw.
func (p *Provider) updateMCPServer(key string, spec map[string]any) error {
	current, err := p.observeMCPServer(key)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("mcp server %s not found for update", key)
	}

	id, ok := current["id"].(string)
	if !ok {
		return fmt.Errorf("mcp server %s: missing id", key)
	}

	body := translateSpec(spec)
	_, err = p.http.Put(context.Background(), "/v1/mcp/servers/"+id, body)
	if errors.Is(err, ErrNotFound) {
		return fmt.Errorf("mcp server %s (id=%s) not found: %w", key, id, err)
	}
	return err
}
