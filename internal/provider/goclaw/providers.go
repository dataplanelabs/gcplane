package goclaw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// observeProvider fetches a provider by name from GoClaw.
// API key is masked as "***" — excluded from returned spec.
func (p *Provider) observeProvider(key string) (map[string]any, error) {
	data, err := p.http.Get(context.Background(), "/v1/providers")
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}

	var resp struct {
		Providers []map[string]any `json:"providers"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse providers response: %w", err)
	}

	for _, prov := range resp.Providers {
		if strVal(prov, "name") == key {
			return translateResult(stripInternal(prov)), nil
		}
	}
	return nil, nil
}

// createProvider creates a new LLM provider in GoClaw.
func (p *Provider) createProvider(key string, spec map[string]any) error {
	body := translateSpec(spec)
	body["name"] = key

	_, err := p.http.Post(context.Background(), "/v1/providers", body)
	if err != nil {
		return fmt.Errorf("create provider %s: %w", key, err)
	}
	return nil
}

// updateProvider updates an existing provider in GoClaw.
// Always sends apiKey from manifest since GoClaw masks it.
func (p *Provider) updateProvider(key string, spec map[string]any) error {
	current, err := p.observeProvider(key)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("provider %s not found for update", key)
	}

	id, ok := current["id"].(string)
	if !ok {
		return fmt.Errorf("provider %s: missing id", key)
	}

	body := translateSpec(spec)
	body["name"] = key

	_, err = p.http.Put(context.Background(), "/v1/providers/"+id, body)
	if errors.Is(err, ErrNotFound) {
		return fmt.Errorf("provider %s (id=%s) not found: %w", key, id, err)
	}
	return err
}
