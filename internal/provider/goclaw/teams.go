package goclaw

import (
	"context"
	"encoding/json"
	"fmt"
)

// observeTeam fetches a team by name via WS RPC.
func (p *Provider) observeTeam(key string) (map[string]any, error) {
	if err := p.ensureWS(); err != nil {
		return nil, fmt.Errorf("ws connect for teams: %w", err)
	}

	payload, err := p.ws.Call(context.Background(), "teams.list", nil)
	if err != nil {
		return nil, fmt.Errorf("teams.list: %w", err)
	}

	var resp struct {
		Teams []map[string]any `json:"teams"`
	}
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, fmt.Errorf("parse teams.list response: %w", err)
	}

	for _, team := range resp.Teams {
		if strVal(team, "name") == key {
			return translateResult(team), nil
		}
	}
	return nil, nil
}

// createTeam creates a new team via WS RPC.
func (p *Provider) createTeam(key string, spec map[string]any) error {
	if err := p.ensureWS(); err != nil {
		return fmt.Errorf("ws connect for teams: %w", err)
	}

	params := translateSpec(spec)
	params["name"] = key

	_, err := p.ws.Call(context.Background(), "teams.create", params)
	if err != nil {
		return fmt.Errorf("teams.create %s: %w", key, err)
	}
	return nil
}

// updateTeam updates an existing team via WS RPC.
func (p *Provider) updateTeam(key string, spec map[string]any) error {
	if err := p.ensureWS(); err != nil {
		return fmt.Errorf("ws connect for teams: %w", err)
	}

	current, err := p.observeTeam(key)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("team %s not found for update", key)
	}

	teamID := strVal(current, "id")
	if teamID == "" {
		teamID = strVal(current, "name")
	}

	params := map[string]any{
		"teamId": teamID,
		"patch":  translateSpec(spec),
	}

	_, err = p.ws.Call(context.Background(), "teams.update", params)
	if err != nil {
		return fmt.Errorf("teams.update %s: %w", key, err)
	}
	return nil
}
