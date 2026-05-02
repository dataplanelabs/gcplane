package goclaw

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// agentLinkInternalFields are link-specific fields returned by GoClaw WS RPC
// (snake_case in JSON tags) that should be stripped from observe results.
// Identity is encoded in the manifest name (sourceKey--targetKey); the
// resolved UUIDs and joined denormalized fields would otherwise cause phantom diffs.
var agentLinkInternalFields = []string{
	"sourceAgentId", "targetAgentId",
	"sourceAgentKey", "sourceDisplayName", "sourceEmoji",
	"targetAgentKey", "targetDisplayName", "targetEmoji", "targetDescription",
	"teamId", "teamName", "targetIsTeamLead", "targetTeamName",
}

// parseAgentLinkName splits a composite link name "sourceAgent--targetAgent"
// into its parts. Returns an error if the format is invalid (missing separator,
// empty halves, or 3+ segments — agent_keys may not contain "--").
func parseAgentLinkName(name string) (sourceKey, targetKey string, err error) {
	parts := strings.Split(name, "--")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid agent link name %q: expected format sourceAgent--targetAgent (agent_keys may not contain %q)", name, "--")
	}
	return parts[0], parts[1], nil
}

// observeAgentLink fetches an agent link by composite name via WS RPC.
// Strategy: resolve sourceKey → sourceID, list links from that source, find target.
func (p *Provider) observeAgentLink(ctx context.Context, key string) (map[string]any, error) {
	if err := p.ensureWS(ctx); err != nil {
		return nil, fmt.Errorf("ws connect for agent links: %w", err)
	}

	sourceKey, targetKey, err := parseAgentLinkName(key)
	if err != nil {
		return nil, err
	}

	sourceID, err := p.resolveAgentID(ctx, sourceKey)
	if err != nil {
		return nil, fmt.Errorf("link %s: source: %w", key, err)
	}
	targetID, err := p.resolveAgentID(ctx, targetKey)
	if err != nil {
		return nil, fmt.Errorf("link %s: target: %w", key, err)
	}

	payload, err := p.ws.Call(ctx, "agents.links.list", map[string]any{
		"agentId":   sourceID,
		"direction": "from",
	})
	if err != nil {
		return nil, fmt.Errorf("agents.links.list: %w", err)
	}

	var resp struct {
		Links []map[string]any `json:"links"`
	}
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, fmt.Errorf("parse agents.links.list response: %w", err)
	}

	for _, link := range resp.Links {
		// AgentLinkData json tags are snake_case; translateResult converts to camelCase.
		camel := translateResult(link)
		if strVal(camel, "targetAgentId") != targetID {
			continue
		}
		// Keep id for update/delete; strip joined denorm fields.
		result := stripInternal(copyMap(link))
		result = translateResult(result)
		for _, f := range agentLinkInternalFields {
			delete(result, f)
		}
		return result, nil
	}
	return nil, nil
}

// createAgentLink creates a new agent link via WS RPC.
// agents.links.* uses camelCase JSON (no translateSpec needed).
func (p *Provider) createAgentLink(ctx context.Context, key string, spec map[string]any) error {
	if err := p.ensureWS(ctx); err != nil {
		return fmt.Errorf("ws connect for agent links: %w", err)
	}

	sourceKey, targetKey, err := parseAgentLinkName(key)
	if err != nil {
		return err
	}

	// sourceAgent / targetAgent in spec must agree with the composite name.
	// If spec provides them, prefer the composite-name keys to avoid divergence.
	params := copyMap(spec)
	params["sourceAgent"] = sourceKey
	params["targetAgent"] = targetKey

	// Strip manifest-only marker keys that aren't part of the create RPC.
	// (settings, direction, description, maxConcurrent pass through directly.)

	if _, err := p.ws.Call(ctx, "agents.links.create", params); err != nil {
		return fmt.Errorf("agents.links.create %s: %w", key, err)
	}
	return nil
}

// updateAgentLink patches an existing agent link via WS RPC.
// agents.links.update takes linkId — Observe first to resolve it.
func (p *Provider) updateAgentLink(ctx context.Context, key string, spec map[string]any) error {
	if err := p.ensureWS(ctx); err != nil {
		return fmt.Errorf("ws connect for agent links: %w", err)
	}

	current, err := p.observeAgentLink(ctx, key)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("agent link %s not found for update", key)
	}

	linkID := strVal(current, "id")
	if linkID == "" {
		return fmt.Errorf("agent link %s: missing id", key)
	}

	patch := copyMap(spec)
	// Source/target are immutable identity — drop if user included them.
	delete(patch, "sourceAgent")
	delete(patch, "targetAgent")
	patch["linkId"] = linkID

	if _, err := p.ws.Call(ctx, "agents.links.update", patch); err != nil {
		return fmt.Errorf("agents.links.update %s: %w", key, err)
	}
	return nil
}

// deleteAgentLink removes an agent link via WS RPC. Idempotent.
func (p *Provider) deleteAgentLink(ctx context.Context, key string) error {
	if err := p.ensureWS(ctx); err != nil {
		return fmt.Errorf("ws connect for agent links: %w", err)
	}

	current, err := p.observeAgentLink(ctx, key)
	if err != nil {
		// Treat agent-not-found as already-gone.
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return err
	}
	if current == nil {
		return nil
	}

	linkID := strVal(current, "id")
	if linkID == "" {
		return fmt.Errorf("agent link %s: missing id", key)
	}

	if _, err := p.ws.Call(ctx, "agents.links.delete", map[string]any{"linkId": linkID}); err != nil {
		return fmt.Errorf("agents.links.delete %s: %w", key, err)
	}
	return nil
}
