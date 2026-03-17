package goclaw

import (
	"context"
	"fmt"
)

// deleteProvider deletes an LLM provider by name. Idempotent: returns nil if not found.
func (p *Provider) deleteProvider(key string) error {
	current, err := p.observeProvider(key)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	id, ok := current["id"].(string)
	if !ok {
		return fmt.Errorf("provider %s: missing id", key)
	}
	return p.http.Delete(context.Background(), "/v1/providers/"+id)
}

// deleteAgent deletes an agent by key. Idempotent: returns nil if not found.
func (p *Provider) deleteAgent(key string) error {
	current, err := p.observeAgent(key)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	id, ok := current["id"].(string)
	if !ok {
		return fmt.Errorf("agent %s: missing id", key)
	}
	return p.http.Delete(context.Background(), "/v1/agents/"+id)
}

// deleteChannelInstance deletes a channel instance by name. Idempotent: returns nil if not found.
func (p *Provider) deleteChannelInstance(key string) error {
	current, err := p.observeChannelInstance(key)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	id, ok := current["id"].(string)
	if !ok {
		return fmt.Errorf("channel instance %s: missing id", key)
	}
	return p.http.Delete(context.Background(), "/v1/channels/instances/"+id)
}

// deleteMCPServer deletes an MCP server by name. Idempotent: returns nil if not found.
func (p *Provider) deleteMCPServer(key string) error {
	current, err := p.observeMCPServer(key)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	id, ok := current["id"].(string)
	if !ok {
		return fmt.Errorf("mcp server %s: missing id", key)
	}
	return p.http.Delete(context.Background(), "/v1/mcp/servers/"+id)
}

// deleteCustomTool deletes a custom tool by name. Idempotent: returns nil if not found.
func (p *Provider) deleteCustomTool(key string) error {
	current, err := p.observeCustomTool(key)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	id, ok := current["id"].(string)
	if !ok {
		return fmt.Errorf("custom tool %s: missing id", key)
	}
	return p.http.Delete(context.Background(), "/v1/tools/custom/"+id)
}

// deleteCronJob deletes a cron job by name via WS RPC. Idempotent: returns nil if not found.
func (p *Provider) deleteCronJob(key string) error {
	if err := p.ensureWS(); err != nil {
		return err
	}
	current, err := p.observeCronJob(key)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	jobID := strVal(current, "id")
	if jobID == "" {
		jobID = strVal(current, "name")
	}
	_, err = p.ws.Call(context.Background(), "cron.delete", map[string]any{"jobId": jobID})
	return err
}

// deleteTeam deletes a team by name via WS RPC. Idempotent: returns nil if not found.
func (p *Provider) deleteTeam(key string) error {
	if err := p.ensureWS(); err != nil {
		return err
	}
	current, err := p.observeTeam(key)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	teamID := strVal(current, "id")
	if teamID == "" {
		teamID = strVal(current, "name")
	}
	_, err = p.ws.Call(context.Background(), "teams.delete", map[string]any{"teamId": teamID})
	return err
}
