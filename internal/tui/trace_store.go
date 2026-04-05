package tui

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/dataplanelabs/gcplane/internal/tui/views"
)

// TraceStore holds LLM agent traces fetched from GoClaw /v1/traces API.
// Thread-safe: WS goroutine signals updates, UI goroutine reads snapshots.
type TraceStore struct {
	mu           sync.RWMutex
	traces       []views.TraceData
	total        int
	filters      views.TraceFilters
	selectedID   string            // currently selected trace ID
	spans        []views.SpanData  // raw spans for selected trace
	spanTree     []*views.SpanNode // built tree for selected trace
	selectedData *views.TraceData  // trace header for selected trace
	listDirty    bool
	detailDirty  bool
	fetching     atomic.Bool // guards against duplicate refresh goroutines
}

// NewTraceStore creates a store with default filters.
func NewTraceStore() *TraceStore {
	return &TraceStore{
		filters: views.TraceFilters{Limit: 50},
	}
}

// NotifyTraceUpdated marks the list as needing refresh.
// Called from WS goroutine — must not block.
func (s *TraceStore) NotifyTraceUpdated() {
	s.mu.Lock()
	s.listDirty = true
	s.mu.Unlock()
}

// SetFilters updates query filters and marks list dirty.
func (s *TraceStore) SetFilters(f views.TraceFilters) {
	s.mu.Lock()
	s.filters = f
	if s.filters.Limit == 0 {
		s.filters.Limit = 50
	}
	s.listDirty = true
	s.mu.Unlock()
}

// Filters returns current filters (snapshot).
func (s *TraceStore) Filters() views.TraceFilters {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.filters
}

// NeedsListRefresh returns true if trace list should be re-fetched.
func (s *TraceStore) NeedsListRefresh() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.listDirty
}

// NotifyDetailDirty marks the detail as needing refresh.
func (s *TraceStore) NotifyDetailDirty() {
	s.mu.Lock()
	s.detailDirty = true
	s.mu.Unlock()
}

// NeedsDetailRefresh returns true if span detail should be re-fetched.
func (s *TraceStore) NeedsDetailRefresh() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.detailDirty
}

// SelectTrace sets the selected trace and marks detail dirty.
func (s *TraceStore) SelectTrace(id string) {
	s.mu.Lock()
	if s.selectedID != id {
		s.selectedID = id
		s.spans = nil
		s.spanTree = nil
		s.selectedData = nil
		s.detailDirty = true
	}
	s.mu.Unlock()
}

// SelectedID returns the currently selected trace ID.
func (s *TraceStore) SelectedID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.selectedID
}

// RefreshList fetches traces via the provided fetcher.
// Runs HTTP call outside lock. Safe to call from goroutine.
// Returns false if another fetch is already in progress.
func (s *TraceStore) RefreshList(ctx context.Context, fetcher TraceFetcher) error {
	if !s.fetching.CompareAndSwap(false, true) {
		return nil // another goroutine is already fetching
	}
	defer s.fetching.Store(false)

	s.mu.RLock()
	f := s.filters
	s.mu.RUnlock()

	traces, total, err := fetcher.ListTraces(ctx, f)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.traces = traces
	s.total = total
	s.listDirty = false
	s.mu.Unlock()
	return nil
}

// RefreshDetail fetches spans for the selected trace.
// Runs HTTP call outside lock. Safe to call from goroutine.
func (s *TraceStore) RefreshDetail(ctx context.Context, fetcher TraceFetcher) error {
	s.mu.RLock()
	id := s.selectedID
	s.mu.RUnlock()

	if id == "" {
		return nil
	}

	trace, spans, err := fetcher.GetTrace(ctx, id)
	if err != nil {
		return err
	}

	tree := views.BuildSpanTree(spans)

	s.mu.Lock()
	// Only update if selection hasn't changed during fetch
	if s.selectedID == id {
		s.selectedData = trace
		s.spans = spans
		s.spanTree = tree
		s.detailDirty = false
	}
	s.mu.Unlock()
	return nil
}

// Traces returns a snapshot of the trace list.
func (s *TraceStore) Traces() []views.TraceData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.traces) == 0 {
		return nil
	}
	out := make([]views.TraceData, len(s.traces))
	copy(out, s.traces)
	return out
}

// Total returns the total trace count from the last fetch.
func (s *TraceStore) Total() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.total
}

// SelectedSpans returns the raw spans for the selected trace.
func (s *TraceStore) SelectedSpans() []views.SpanData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.spans) == 0 {
		return nil
	}
	out := make([]views.SpanData, len(s.spans))
	copy(out, s.spans)
	return out
}

// SelectedSpanTree returns the built span tree for the selected trace.
func (s *TraceStore) SelectedSpanTree() []*views.SpanNode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.spanTree
}

// SelectedTrace returns the trace header data for the selected trace.
func (s *TraceStore) SelectedTrace() *views.TraceData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.selectedData
}
