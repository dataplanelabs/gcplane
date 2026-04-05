package goclaw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// observeSecureCLI fetches a secure CLI binary by binary_name from GoClaw.
func (p *Provider) observeSecureCLI(ctx context.Context, key string) (map[string]any, error) {
	data, err := p.http.Get(ctx, "/v1/cli-credentials")
	if err != nil {
		return nil, fmt.Errorf("list cli-credentials: %w", err)
	}

	var resp struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse cli-credentials response: %w", err)
	}

	for _, item := range resp.Items {
		if strVal(item, "binary_name") == key && p.matchesTenant(ctx, item) {
			// Strip env_keys (read-only computed field, not in manifest)
			delete(item, "env_keys")
			delete(item, "encrypted_env")
			return translateResult(stripInternal(item)), nil
		}
	}
	return nil, nil
}

// createSecureCLI creates a new secure CLI binary in GoClaw.
func (p *Provider) createSecureCLI(ctx context.Context, key string, spec map[string]any) error {
	body := translateSpec(spec)
	body["binary_name"] = key

	_, err := p.http.Post(ctx, "/v1/cli-credentials", body)
	if err != nil {
		return fmt.Errorf("create cli-credential %s: %w", key, err)
	}
	return nil
}

// updateSecureCLI updates an existing secure CLI binary in GoClaw.
func (p *Provider) updateSecureCLI(ctx context.Context, key string, spec map[string]any) error {
	current, err := p.observeSecureCLI(ctx, key)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("cli-credential %s not found for update", key)
	}

	id, ok := current["id"].(string)
	if !ok {
		return fmt.Errorf("cli-credential %s: missing id", key)
	}

	body := translateSpec(spec)
	_, err = p.http.Put(ctx, "/v1/cli-credentials/"+id, body)
	if errors.Is(err, ErrNotFound) {
		return fmt.Errorf("cli-credential %s (id=%s) not found: %w", key, id, err)
	}
	return err
}

// deleteSecureCLI deletes a secure CLI binary by name. Idempotent.
func (p *Provider) deleteSecureCLI(ctx context.Context, key string) error {
	current, err := p.observeSecureCLI(ctx, key)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	id, ok := current["id"].(string)
	if !ok {
		return fmt.Errorf("cli-credential %s: missing id", key)
	}
	return p.http.Delete(ctx, "/v1/cli-credentials/"+id)
}

// resolveSecureCLIID looks up a secure CLI binary by name and returns its UUID.
func (p *Provider) resolveSecureCLIID(ctx context.Context, binaryName string) (string, error) {
	data, err := p.http.Get(ctx, "/v1/cli-credentials")
	if err != nil {
		return "", fmt.Errorf("list cli-credentials: %w", err)
	}

	var resp struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("parse cli-credentials response: %w", err)
	}

	for _, item := range resp.Items {
		if strVal(item, "binary_name") == binaryName && p.matchesTenant(ctx, item) {
			if id := strVal(item, "id"); id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("cli-credential %q not found", binaryName)
}
