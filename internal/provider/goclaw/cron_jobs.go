package goclaw

import (
	"context"
	"encoding/json"
	"fmt"
)

// cronInternalFields are CronJob-specific fields returned by GoClaw WS RPC (camelCase)
// that should be stripped from observe results. The generic internalFields list uses
// snake_case and doesn't match these.
// Note: "createdBy" is defensive — GoClaw's CronJob struct currently lacks this field,
// but stripping it prevents phantom diffs if GoClaw adds it later.
var cronInternalFields = []string{
	"tenantId", "userId", "payload", "state", "createdAtMs", "updatedAtMs", "createdBy",
}

// observeCronJob fetches a cron job by name via WS RPC.
func (p *Provider) observeCronJob(ctx context.Context, key string) (map[string]any, error) {
	if err := p.ensureWS(ctx); err != nil {
		return nil, fmt.Errorf("ws connect for cron: %w", err)
	}

	// includeDisabled: cron.list omits disabled jobs by default. Without this,
	// a manifest cron with enabled=false is never observed, so the reconciler
	// re-issues CREATE every cycle and hits the (agent,tenant,name) unique key.
	payload, err := p.ws.Call(ctx, "cron.list", map[string]any{"includeDisabled": true})
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
			result := translateResult(stripInternal(job))
			// Strip CronJob-specific internal fields (WS RPC returns camelCase,
			// which stripInternal's snake_case list doesn't catch).
			for _, f := range cronInternalFields {
				delete(result, f)
			}
			return result, nil
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
