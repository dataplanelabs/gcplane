package goclaw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
)

// sysConfigPath builds a safe URL path for a system config key.
// Escapes the key to prevent path traversal or injection.
func sysConfigPath(key string) string {
	return "/v1/system-configs/" + url.PathEscape(key)
}

// observeSystemConfig fetches a system config by key from GoClaw.
// Returns nil if the key does not exist (not-found is not an error).
func (p *Provider) observeSystemConfig(ctx context.Context, key string) (map[string]any, error) {
	data, err := p.http.Get(ctx, sysConfigPath(key))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("observe system config %q: %w", key, err)
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse system config %q response: %w", key, err)
	}

	return map[string]any{"value": config["value"]}, nil
}

// createSystemConfig sets a system config key-value pair (same as update — PUT is upsert).
func (p *Provider) createSystemConfig(ctx context.Context, key string, spec map[string]any) error {
	return p.updateSystemConfig(ctx, key, spec)
}

// updateSystemConfig updates a system config key-value pair via PUT.
func (p *Provider) updateSystemConfig(ctx context.Context, key string, spec map[string]any) error {
	value, _ := spec["value"].(string)

	_, err := p.http.Put(ctx, sysConfigPath(key), map[string]any{"value": value})
	if err != nil {
		return fmt.Errorf("update system config %q: %w", key, err)
	}
	return nil
}

// deleteSystemConfig removes a system config key. Idempotent: no-op if already absent.
func (p *Provider) deleteSystemConfig(ctx context.Context, key string) error {
	err := p.http.Delete(ctx, sysConfigPath(key))
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete system config %q: %w", key, err)
	}
	return nil
}
