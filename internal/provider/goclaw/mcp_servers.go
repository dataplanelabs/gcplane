package goclaw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// observeMCPServer fetches an MCP server by key from GoClaw.
// It also fetches current agent grants and injects them as grants.agents
// so the reconciler can detect grant drift.
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
			serverID := strVal(s, "id")
			result := translateResult(stripInternal(s))
			// Inject live grant list so reconciler can compare against desired.
			if serverID != "" {
				if agentKeys, err := p.listMCPGrantAgentKeys(serverID); err == nil {
					result["grants"] = map[string]any{"agents": agentKeys}
				}
			}
			return result, nil
		}
	}
	return nil, nil
}

// createMCPServer creates a new MCP server in GoClaw then applies agent grants.
func (p *Provider) createMCPServer(key string, spec map[string]any) error {
	body := translateSpec(spec)
	body["name"] = key
	// grants are managed via a separate API — strip from create body
	delete(body, "grants")

	_, err := p.http.Post(context.Background(), "/v1/mcp/servers", body)
	if err != nil {
		return fmt.Errorf("create mcp server %s: %w", key, err)
	}

	if agents := extractGrantAgents(spec); len(agents) > 0 {
		if err := p.applyMCPGrants(key, agents); err != nil {
			return fmt.Errorf("apply grants for mcp server %s: %w", key, err)
		}
	}
	return nil
}

// updateMCPServer updates an existing MCP server in GoClaw then reconciles agent grants.
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
	delete(body, "grants")
	_, err = p.http.Put(context.Background(), "/v1/mcp/servers/"+id, body)
	if errors.Is(err, ErrNotFound) {
		return fmt.Errorf("mcp server %s (id=%s) not found: %w", key, id, err)
	}
	if err != nil {
		return err
	}

	return p.applyMCPGrants(key, extractGrantAgents(spec))
}

// applyMCPGrants reconciles desired agent grants for an MCP server:
// adds missing grants and removes extra ones.
func (p *Provider) applyMCPGrants(serverName string, desiredAgents []string) error {
	serverID, err := p.resolveMCPServerID(serverName)
	if err != nil {
		return err
	}

	currentGrants, err := p.listMCPGrants(serverID)
	if err != nil {
		return err
	}

	// Map agentID → present for current grants.
	currentIDs := make(map[string]struct{}, len(currentGrants))
	for _, g := range currentGrants {
		if id := strVal(g, "agent_id"); id != "" {
			currentIDs[id] = struct{}{}
		}
	}

	// Resolve desired agent names → IDs.
	desiredIDs := make(map[string]struct{}, len(desiredAgents))
	for _, agentKey := range desiredAgents {
		agentID, err := p.resolveAgentID(agentKey)
		if err != nil {
			return fmt.Errorf("resolve agent %q: %w", agentKey, err)
		}
		desiredIDs[agentID] = struct{}{}
	}

	// Add missing grants.
	for agentID := range desiredIDs {
		if _, exists := currentIDs[agentID]; !exists {
			body := map[string]any{"agent_id": agentID}
			if _, err := p.http.Post(context.Background(), "/v1/mcp/servers/"+serverID+"/grants/agent", body); err != nil {
				return fmt.Errorf("grant agent %s to mcp server %s: %w", agentID, serverName, err)
			}
		}
	}

	// Remove extra grants.
	for agentID := range currentIDs {
		if _, wanted := desiredIDs[agentID]; !wanted {
			path := "/v1/mcp/servers/" + serverID + "/grants/agent/" + agentID
			if err := p.http.Delete(context.Background(), path); err != nil && !errors.Is(err, ErrNotFound) {
				return fmt.Errorf("revoke agent %s from mcp server %s: %w", agentID, serverName, err)
			}
		}
	}

	return nil
}

// resolveMCPServerID returns the UUID of an MCP server by its name.
func (p *Provider) resolveMCPServerID(name string) (string, error) {
	data, err := p.http.Get(context.Background(), "/v1/mcp/servers")
	if err != nil {
		return "", fmt.Errorf("list mcp servers: %w", err)
	}

	var resp struct {
		Servers []map[string]any `json:"servers"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("parse mcp servers response: %w", err)
	}

	for _, s := range resp.Servers {
		if strVal(s, "name") == name {
			if id := strVal(s, "id"); id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("mcp server %q not found", name)
}

// listMCPGrants returns raw grant objects for a server (each has at minimum "agent_id").
func (p *Provider) listMCPGrants(serverID string) ([]map[string]any, error) {
	data, err := p.http.Get(context.Background(), "/v1/mcp/servers/"+serverID+"/grants")
	if err != nil {
		return nil, fmt.Errorf("list mcp grants for server %s: %w", serverID, err)
	}

	var resp struct {
		Grants []map[string]any `json:"grants"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse mcp grants response: %w", err)
	}

	return resp.Grants, nil
}

// listMCPGrantAgentKeys resolves grant agent UUIDs back to agent keys
// so the reconciler can compare them against manifest names.
func (p *Provider) listMCPGrantAgentKeys(serverID string) ([]string, error) {
	grants, err := p.listMCPGrants(serverID)
	if err != nil {
		return nil, err
	}
	if len(grants) == 0 {
		return []string{}, nil
	}

	// Fetch agents once to map IDs → keys.
	agentData, err := p.http.Get(context.Background(), "/v1/agents")
	if err != nil {
		return nil, fmt.Errorf("list agents for grant resolution: %w", err)
	}
	var agentResp struct {
		Agents []map[string]any `json:"agents"`
	}
	if err := json.Unmarshal(agentData, &agentResp); err != nil {
		return nil, fmt.Errorf("parse agents response: %w", err)
	}
	idToKey := make(map[string]string, len(agentResp.Agents))
	for _, a := range agentResp.Agents {
		if id := strVal(a, "id"); id != "" {
			idToKey[id] = strVal(a, "agent_key")
		}
	}

	keys := make([]string, 0, len(grants))
	for _, g := range grants {
		agentID := strVal(g, "agent_id")
		if key, ok := idToKey[agentID]; ok && key != "" {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

// extractGrantAgents extracts the grants.agents string slice from a manifest spec.
func extractGrantAgents(spec map[string]any) []string {
	grants, ok := spec["grants"].(map[string]any)
	if !ok {
		return nil
	}
	raw, ok := grants["agents"]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}
