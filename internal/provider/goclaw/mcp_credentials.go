package goclaw

import (
	"context"
	"fmt"
)

// observeMCPCredentials checks for per-user credentials on an MCP server.
// Credentials are encrypted and may not be fully observable — returns a
// minimal stub so the reconciler relies on WriteOnlyFields for credentials.
func (p *Provider) observeMCPCredentials(key string) (map[string]any, error) {
	// Verify the MCP server exists; if not, credentials can't exist either
	_, err := p.resolveMCPServerID(key)
	if err != nil {
		return nil, nil // server not found = credentials not set
	}
	// Return a stub — actual credentials are write-only (encrypted at rest)
	result := map[string]any{
		"serverName": key,
	}
	return result, nil
}

// createMCPCredentials sets per-user credentials for an MCP server.
func (p *Provider) createMCPCredentials(key string, spec map[string]any) error {
	id, err := p.resolveMCPServerID(key)
	if err != nil {
		return err
	}

	body := translateSpec(spec)
	path := fmt.Sprintf("/v1/mcp/servers/%s/user-credentials", id)
	_, err = p.http.Put(context.Background(), path, body)
	if err != nil {
		return fmt.Errorf("set mcp credentials %s: %w", key, err)
	}
	return nil
}

// updateMCPCredentials is the same as create — PUT is upsert.
func (p *Provider) updateMCPCredentials(key string, spec map[string]any) error {
	return p.createMCPCredentials(key, spec)
}

// deleteMCPCredentials removes per-user credentials for an MCP server.
func (p *Provider) deleteMCPCredentials(key string) error {
	id, err := p.resolveMCPServerID(key)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/v1/mcp/servers/%s/user-credentials", id)
	return p.http.Delete(context.Background(), path)
}
