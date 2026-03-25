package goclaw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// observeTenant fetches a tenant by slug from GoClaw.
func (p *Provider) observeTenant(ctx context.Context, key string) (map[string]any, error) {
	data, err := p.http.Get(ctx, "/v1/tenants")
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}

	var resp struct {
		Tenants []map[string]any `json:"tenants"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse tenants response: %w", err)
	}

	for _, t := range resp.Tenants {
		if strVal(t, "slug") == key {
			return translateResult(stripInternal(t)), nil
		}
	}
	return nil, nil
}

// createTenant creates a new tenant in GoClaw.
func (p *Provider) createTenant(ctx context.Context, key string, spec map[string]any) error {
	body := translateSpec(spec)
	body["slug"] = key

	_, err := p.http.Post(ctx, "/v1/tenants", body)
	if err != nil {
		return fmt.Errorf("create tenant %s: %w", key, err)
	}
	return nil
}

// updateTenant updates an existing tenant in GoClaw.
func (p *Provider) updateTenant(ctx context.Context, key string, spec map[string]any) error {
	current, err := p.observeTenant(ctx, key)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("tenant %s not found for update", key)
	}

	id, ok := current["id"].(string)
	if !ok {
		return fmt.Errorf("tenant %s: missing id", key)
	}

	body := translateSpec(spec)
	_, err = p.http.Put(ctx, "/v1/tenants/"+id, body)
	if errors.Is(err, ErrNotFound) {
		return fmt.Errorf("tenant %s (id=%s) not found: %w", key, id, err)
	}
	return err
}

// deleteTenant deletes a tenant from GoClaw. Idempotent: no-op if already absent.
func (p *Provider) deleteTenant(ctx context.Context, key string) error {
	current, err := p.observeTenant(ctx, key)
	if err != nil {
		return err
	}
	if current == nil {
		return nil // idempotent
	}

	id, ok := current["id"].(string)
	if !ok {
		return fmt.Errorf("tenant %s: missing id for delete", key)
	}
	return p.http.Delete(ctx, "/v1/tenants/"+id)
}
