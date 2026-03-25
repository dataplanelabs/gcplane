package goclaw

import (
	"context"
	"fmt"
)

// deleteProvider deletes an LLM provider by name. Idempotent: returns nil if not found.
func (p *Provider) deleteProvider(ctx context.Context, key string) error {
	current, err := p.observeProvider(ctx, key)
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
	return p.http.Delete(ctx, "/v1/providers/"+id)
}

// deleteAgent deletes an agent by key. Idempotent: returns nil if not found.
func (p *Provider) deleteAgent(ctx context.Context, key string) error {
	current, err := p.observeAgent(ctx, key)
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
	return p.http.Delete(ctx, "/v1/agents/"+id)
}

// deleteChannelInstance deletes a channel instance by name. Idempotent: returns nil if not found.
func (p *Provider) deleteChannelInstance(ctx context.Context, key string) error {
	current, err := p.observeChannelInstance(ctx, key)
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
	return p.http.Delete(ctx, "/v1/channels/instances/"+id)
}

// deleteMCPServer deletes an MCP server by name. Idempotent: returns nil if not found.
func (p *Provider) deleteMCPServer(ctx context.Context, key string) error {
	current, err := p.observeMCPServer(ctx, key)
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
	return p.http.Delete(ctx, "/v1/mcp/servers/"+id)
}

// deleteCustomTool deletes a custom tool by name. Idempotent: returns nil if not found.
func (p *Provider) deleteCustomTool(ctx context.Context, key string) error {
	current, err := p.observeCustomTool(ctx, key)
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
	return p.http.Delete(ctx, "/v1/tools/custom/"+id)
}

// deleteCronJob deletes a cron job by name via WS RPC. Idempotent: returns nil if not found.
func (p *Provider) deleteCronJob(ctx context.Context, key string) error {
	if err := p.ensureWS(ctx); err != nil {
		return err
	}
	current, err := p.observeCronJob(ctx, key)
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
	_, err = p.ws.Call(ctx, "cron.delete", map[string]any{"jobId": jobID})
	return err
}

// deleteTeam deletes a team by name via WS RPC. Idempotent: returns nil if not found.
func (p *Provider) deleteTeam(ctx context.Context, key string) error {
	if err := p.ensureWS(ctx); err != nil {
		return err
	}
	current, err := p.observeTeam(ctx, key)
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
	_, err = p.ws.Call(ctx, "teams.delete", map[string]any{"teamId": teamID})
	return err
}
