package goclaw

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// grantInternalFields are fields specific to grants that should be stripped
// from observe results (in addition to the standard internalFields).
var grantInternalFields = []string{
	"binary_id", "agent_id",
}

// parseGrantName splits a composite grant name "binaryName--agentKey" into parts.
// Returns an error if the format is invalid.
func parseGrantName(name string) (binaryName, agentKey string, err error) {
	before, after, ok := strings.Cut(name, "--")
	if !ok || before == "" || after == "" {
		return "", "", fmt.Errorf("invalid grant name %q: expected format binaryName--agentKey", name)
	}
	return before, after, nil
}

// observeSecureCLIGrant fetches a grant by resolving the composite key.
func (p *Provider) observeSecureCLIGrant(ctx context.Context, key string) (map[string]any, error) {
	binaryName, agentKey, err := parseGrantName(key)
	if err != nil {
		return nil, err
	}

	binaryID, err := p.resolveSecureCLIID(ctx, binaryName)
	if err != nil {
		return nil, fmt.Errorf("grant %s: %w", key, err)
	}

	agentID, err := p.resolveAgentID(ctx, agentKey)
	if err != nil {
		return nil, fmt.Errorf("grant %s: %w", key, err)
	}

	data, err := p.http.Get(ctx, "/v1/cli-credentials/"+binaryID+"/agent-grants")
	if err != nil {
		return nil, fmt.Errorf("list grants for %s: %w", binaryName, err)
	}

	var resp struct {
		Grants []map[string]any `json:"grants"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse grants response: %w", err)
	}

	for _, g := range resp.Grants {
		if strVal(g, "agent_id") == agentID {
			for _, f := range grantInternalFields {
				delete(g, f)
			}
			return translateResult(stripInternal(g)), nil
		}
	}
	return nil, nil
}

// createSecureCLIGrant creates a new per-agent grant for a secure CLI binary.
func (p *Provider) createSecureCLIGrant(ctx context.Context, key string, spec map[string]any) error {
	binaryName, agentKey, err := parseGrantName(key)
	if err != nil {
		return err
	}

	binaryID, err := p.resolveSecureCLIID(ctx, binaryName)
	if err != nil {
		return fmt.Errorf("grant %s: %w", key, err)
	}

	agentID, err := p.resolveAgentID(ctx, agentKey)
	if err != nil {
		return fmt.Errorf("grant %s: %w", key, err)
	}

	body := translateSpec(spec)
	// Remove manifest-only reference fields; set resolved UUID
	delete(body, "binary_name")
	delete(body, "agent_key")
	body["agent_id"] = agentID

	_, err = p.http.Post(ctx, "/v1/cli-credentials/"+binaryID+"/agent-grants", body)
	if err != nil {
		return fmt.Errorf("create grant %s: %w", key, err)
	}
	return nil
}

// updateSecureCLIGrant updates an existing per-agent grant.
func (p *Provider) updateSecureCLIGrant(ctx context.Context, key string, spec map[string]any) error {
	binaryName, _, err := parseGrantName(key)
	if err != nil {
		return err
	}

	binaryID, err := p.resolveSecureCLIID(ctx, binaryName)
	if err != nil {
		return fmt.Errorf("grant %s: %w", key, err)
	}

	// Observe to get the grant ID
	current, err := p.observeSecureCLIGrant(ctx, key)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("grant %s not found for update", key)
	}

	grantID, ok := current["id"].(string)
	if !ok {
		return fmt.Errorf("grant %s: missing id", key)
	}

	body := translateSpec(spec)
	delete(body, "binary_name")
	delete(body, "agent_key")

	_, err = p.http.Put(ctx, "/v1/cli-credentials/"+binaryID+"/agent-grants/"+grantID, body)
	return err
}

// deleteSecureCLIGrant deletes a per-agent grant. Idempotent.
func (p *Provider) deleteSecureCLIGrant(ctx context.Context, key string) error {
	binaryName, _, err := parseGrantName(key)
	if err != nil {
		return err
	}

	binaryID, err := p.resolveSecureCLIID(ctx, binaryName)
	if err != nil {
		return nil // binary not found → grant already gone
	}

	current, err := p.observeSecureCLIGrant(ctx, key)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}

	grantID, ok := current["id"].(string)
	if !ok {
		return fmt.Errorf("grant %s: missing id", key)
	}

	return p.http.Delete(ctx, "/v1/cli-credentials/"+binaryID+"/agent-grants/"+grantID)
}
