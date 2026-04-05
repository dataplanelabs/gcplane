package views

import (
	"sort"
	"time"

	"github.com/gdamore/tcell/v2"
)

// TraceData represents an LLM agent trace from GoClaw /v1/traces API.
type TraceData struct {
	ID                string         `json:"id"`
	ParentTraceID     string         `json:"parent_trace_id,omitempty"`
	AgentID           string         `json:"agent_id,omitempty"`
	Name              string         `json:"name"`
	Channel           string         `json:"channel"`
	SessionKey        string         `json:"session_key"`
	RunID             string         `json:"run_id"`
	Status            string         `json:"status"` // ok, error, running
	DurationMs        int            `json:"duration_ms"`
	TotalInputTokens  int            `json:"total_input_tokens"`
	TotalOutputTokens int            `json:"total_output_tokens"`
	SpanCount         int            `json:"span_count"`
	LLMCallCount      int            `json:"llm_call_count"`
	ToolCallCount     int            `json:"tool_call_count"`
	InputPreview      string         `json:"input_preview"`
	OutputPreview     string         `json:"output_preview"`
	Error             string         `json:"error,omitempty"`
	StartTime         time.Time      `json:"start_time"`
	EndTime           *time.Time     `json:"end_time,omitempty"`
	Metadata          *TraceMetadata `json:"metadata,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
}

// TraceMetadata holds optional trace-level token cache metrics.
type TraceMetadata struct {
	TotalCacheReadTokens     int `json:"total_cache_read_tokens,omitempty"`
	TotalCacheCreationTokens int `json:"total_cache_creation_tokens,omitempty"`
}

// SpanData represents a single span within a trace.
type SpanData struct {
	ID            string        `json:"id"`
	TraceID       string        `json:"trace_id"`
	ParentSpanID  string        `json:"parent_span_id,omitempty"`
	AgentID       string        `json:"agent_id,omitempty"`
	SpanType      string        `json:"span_type"` // agent, llm_call, tool_call
	Name          string        `json:"name"`
	Status        string        `json:"status"`
	DurationMs    int           `json:"duration_ms"`
	Model         string        `json:"model,omitempty"`
	Provider      string        `json:"provider,omitempty"`
	InputTokens   int           `json:"input_tokens"`
	OutputTokens  int           `json:"output_tokens"`
	ToolName      string        `json:"tool_name,omitempty"`
	InputPreview  string        `json:"input_preview"`
	OutputPreview string        `json:"output_preview"`
	Error         string        `json:"error,omitempty"`
	Metadata      *SpanMetadata `json:"metadata,omitempty"`
	StartTime     time.Time     `json:"start_time"`
	EndTime       *time.Time    `json:"end_time,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
}

// SpanMetadata holds optional span-level metrics (tokens, reasoning).
type SpanMetadata struct {
	CacheCreationTokens int `json:"cache_creation_tokens,omitempty"`
	CacheReadTokens     int `json:"cache_read_tokens,omitempty"`
	ThinkingTokens      int `json:"thinking_tokens,omitempty"`
}

// SpanNode is a tree node wrapping a span and its children.
type SpanNode struct {
	Span     SpanData
	Children []*SpanNode
}

// TraceFilters controls the /v1/traces list query parameters.
type TraceFilters struct {
	AgentID string
	Channel string
	Limit   int
	Offset  int
}

// BuildSpanTree builds a forest of SpanNode trees from a flat span list.
// Roots are spans with no ParentSpanID. Sorted by StartTime for stable ordering.
func BuildSpanTree(spans []SpanData) []*SpanNode {
	nodes := make(map[string]*SpanNode, len(spans))
	for i := range spans {
		nodes[spans[i].ID] = &SpanNode{Span: spans[i]}
	}
	var roots []*SpanNode
	for _, node := range nodes {
		if node.Span.ParentSpanID == "" {
			roots = append(roots, node)
			continue
		}
		if parent, ok := nodes[node.Span.ParentSpanID]; ok {
			parent.Children = append(parent.Children, node)
		} else {
			roots = append(roots, node) // orphan → promote to root
		}
	}
	// Sort for deterministic ordering (prevents tree flicker on refresh)
	sortSpanNodes(roots)
	return roots
}

// sortSpanNodes recursively sorts span nodes by StartTime.
func sortSpanNodes(nodes []*SpanNode) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Span.StartTime.Before(nodes[j].Span.StartTime)
	})
	for _, n := range nodes {
		if len(n.Children) > 1 {
			sortSpanNodes(n.Children)
		}
	}
}

// spanStatusIcon returns a Unicode icon for span status.
func spanStatusIcon(status string) string {
	switch status {
	case "ok", "success", "completed":
		return "\u25cf" // ●
	case "error", "failed":
		return "\u2717" // ✗
	case "running", "pending":
		return "\u25d0" // ◐
	default:
		return "\u25cb" // ○
	}
}

// spanTypeColor returns the Catppuccin color for a span type.
func spanTypeColor(spanType string) tcell.Color {
	switch spanType {
	case "agent":
		return ColorMauve
	case "llm_call":
		return ColorBlue
	case "tool_call":
		return ColorTeal
	default:
		return ColorOverlay0
	}
}
