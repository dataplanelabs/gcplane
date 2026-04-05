package goclaw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/dataplanelabs/gcplane/internal/tui/views"
)

// ListTraces fetches traces from GoClaw with optional filters.
func (p *Provider) ListTraces(ctx context.Context, f views.TraceFilters) ([]views.TraceData, int, error) {
	path := "/v1/traces" + buildTraceQuery(f)
	data, err := p.http.Get(ctx, path)
	if err != nil {
		return nil, 0, fmt.Errorf("list traces: %w", err)
	}

	var resp struct {
		Traces []views.TraceData `json:"traces"`
		Total  int               `json:"total"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, 0, fmt.Errorf("parse traces: %w", err)
	}
	return resp.Traces, resp.Total, nil
}

// GetTrace fetches a single trace with its spans.
func (p *Provider) GetTrace(ctx context.Context, traceID string) (*views.TraceData, []views.SpanData, error) {
	data, err := p.http.Get(ctx, "/v1/traces/"+traceID)
	if err != nil {
		return nil, nil, fmt.Errorf("get trace %s: %w", traceID, err)
	}

	var resp struct {
		Trace views.TraceData  `json:"trace"`
		Spans []views.SpanData `json:"spans"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, nil, fmt.Errorf("parse trace detail: %w", err)
	}
	return &resp.Trace, resp.Spans, nil
}

// buildTraceQuery builds URL query string from filters.
func buildTraceQuery(f views.TraceFilters) string {
	params := url.Values{}
	if f.AgentID != "" {
		params.Set("agent_id", f.AgentID)
	}
	if f.Channel != "" {
		params.Set("channel", f.Channel)
	}
	if f.Limit > 0 {
		params.Set("limit", strconv.Itoa(f.Limit))
	}
	if f.Offset > 0 {
		params.Set("offset", strconv.Itoa(f.Offset))
	}
	if len(params) == 0 {
		return ""
	}
	return "?" + params.Encode()
}
