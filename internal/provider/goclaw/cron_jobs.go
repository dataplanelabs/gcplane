package goclaw

import (
	"context"
	"encoding/json"
	"fmt"
)

// observeCronJob fetches a cron job by name via WS RPC.
func (p *Provider) observeCronJob(key string) (map[string]any, error) {
	if err := p.ensureWS(); err != nil {
		return nil, fmt.Errorf("ws connect for cron: %w", err)
	}

	payload, err := p.ws.Call(context.Background(), "cron.list", nil)
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
		if strVal(job, "name") == key {
			// agentKey/message are write-only; excluded via WriteOnlyFields(KindCronJob).
			return translateResult(stripInternal(job)), nil
		}
	}
	return nil, nil
}

// createCronJob creates a new cron job via WS RPC.
func (p *Provider) createCronJob(key string, spec map[string]any) error {
	if err := p.ensureWS(); err != nil {
		return fmt.Errorf("ws connect for cron: %w", err)
	}

	params := translateSpec(spec)
	params["name"] = key

	_, err := p.ws.Call(context.Background(), "cron.create", params)
	if err != nil {
		return fmt.Errorf("cron.create %s: %w", key, err)
	}
	return nil
}

// updateCronJob updates an existing cron job via WS RPC.
func (p *Provider) updateCronJob(key string, spec map[string]any) error {
	if err := p.ensureWS(); err != nil {
		return fmt.Errorf("ws connect for cron: %w", err)
	}

	current, err := p.observeCronJob(key)
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

	params := map[string]any{
		"jobId": jobID,
		"patch": translateSpec(spec),
	}

	_, err = p.ws.Call(context.Background(), "cron.update", params)
	if err != nil {
		return fmt.Errorf("cron.update %s: %w", key, err)
	}
	return nil
}
