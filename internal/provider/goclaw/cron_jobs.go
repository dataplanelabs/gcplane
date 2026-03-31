package goclaw

import (
	"context"
	"encoding/json"
	"fmt"
)

// observeCronJob fetches a cron job by name via WS RPC.
func (p *Provider) observeCronJob(ctx context.Context, key string) (map[string]any, error) {
	if err := p.ensureWS(ctx); err != nil {
		return nil, fmt.Errorf("ws connect for cron: %w", err)
	}

	payload, err := p.ws.Call(ctx, "cron.list", nil)
	if err != nil {
		return nil, fmt.Errorf("cron.list: %w", err)
	}

	var resp struct {
		Jobs []map[string]any `json:"jobs"`
	}
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, fmt.Errorf("parse cron.list response: %w", err)
	}

	for _, job := range resp.Jobs {
		if strVal(job, "name") == key && p.matchesTenant(ctx, job) {
			// agentKey/message are write-only; excluded via WriteOnlyFields(KindCronJob).
			return translateResult(stripInternal(job)), nil
		}
	}
	return nil, nil
}

// createCronJob creates a new cron job via WS RPC.
// Note: GoClaw's cron RPC uses camelCase JSON (unlike HTTP resources which use snake_case),
// so we pass the manifest spec directly without snake_case translation.
func (p *Provider) createCronJob(ctx context.Context, key string, spec map[string]any) error {
	if err := p.ensureWS(ctx); err != nil {
		return fmt.Errorf("ws connect for cron: %w", err)
	}

	params := copyMap(spec)
	params["name"] = key

	// Resolve agentKey → agentId (GoClaw expects UUID)
	if agentKey, ok := params["agentKey"].(string); ok {
		agentID, err := p.resolveAgentID(ctx, agentKey)
		if err != nil {
			return fmt.Errorf("cron %s: %w", key, err)
		}
		params["agentId"] = agentID
		delete(params, "agentKey")
	}

	_, err := p.ws.Call(ctx, "cron.create", params)
	if err != nil {
		return fmt.Errorf("cron.create %s: %w", key, err)
	}
	return nil
}

// updateCronJob updates an existing cron job via WS RPC.
// Note: GoClaw's cron RPC uses camelCase JSON, so no snake_case translation needed.
func (p *Provider) updateCronJob(ctx context.Context, key string, spec map[string]any) error {
	if err := p.ensureWS(ctx); err != nil {
		return fmt.Errorf("ws connect for cron: %w", err)
	}

	current, err := p.observeCronJob(ctx, key)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("cron job %s not found for update", key)
	}

	jobID := strVal(current, "id")
	if jobID == "" {
		jobID = strVal(current, "name")
	}

	patch := copyMap(spec)

	// Resolve agentKey → agentId (GoClaw expects UUID)
	if agentKey, ok := patch["agentKey"].(string); ok {
		agentID, err := p.resolveAgentID(ctx, agentKey)
		if err != nil {
			return fmt.Errorf("cron %s: %w", key, err)
		}
		patch["agentId"] = agentID
		delete(patch, "agentKey")
	}

	params := map[string]any{
		"jobId": jobID,
		"patch": patch,
	}

	_, err = p.ws.Call(ctx, "cron.update", params)
	if err != nil {
		return fmt.Errorf("cron.update %s: %w", key, err)
	}
	return nil
}
