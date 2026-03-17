package goclaw

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dataplanelabs/gcplane/internal/keyconv"
)

// translateSpec converts manifest camelCase keys to GoClaw snake_case for API calls.
func translateSpec(spec map[string]any) map[string]any {
	return keyconv.CamelToSnake(spec)
}

// translateResult converts GoClaw snake_case keys to manifest camelCase for comparison.
func translateResult(result map[string]any) map[string]any {
	return keyconv.SnakeToCamel(result)
}

// resolveAgentID looks up an agent by key and returns its UUID.
func (p *Provider) resolveAgentID(agentKey string) (string, error) {
	data, err := p.http.Get(context.Background(), "/v1/agents")
	if err != nil {
		return "", fmt.Errorf("list agents: %w", err)
	}

	var resp struct {
		Agents []map[string]any `json:"agents"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("parse agents response: %w", err)
	}

	for _, a := range resp.Agents {
		if strVal(a, "agent_key") == agentKey {
			if id := strVal(a, "id"); id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("agent %q not found", agentKey)
}

// strVal safely extracts a string value from a map.
func strVal(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// copyMap creates a shallow copy of a map.
func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
