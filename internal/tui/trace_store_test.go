package tui

import (
	"context"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/tui/views"
)

// mockTraceFetcher implements TraceFetcher for testing.
type mockTraceFetcher struct {
	traces []views.TraceData
	total  int
	trace  *views.TraceData
	spans  []views.SpanData
	err    error
}

func (m *mockTraceFetcher) ListTraces(_ context.Context, _ views.TraceFilters) ([]views.TraceData, int, error) {
	return m.traces, m.total, m.err
}

func (m *mockTraceFetcher) GetTrace(_ context.Context, _ string) (*views.TraceData, []views.SpanData, error) {
	return m.trace, m.spans, m.err
}

func TestTraceStore_NotifyAndRefresh(t *testing.T) {
	s := NewTraceStore()

	if s.NeedsListRefresh() {
		t.Fatal("new store should not need refresh")
	}

	s.NotifyTraceUpdated()
	if !s.NeedsListRefresh() {
		t.Fatal("should need refresh after notify")
	}

	fetcher := &mockTraceFetcher{
		traces: []views.TraceData{{ID: "t1", Name: "test"}},
		total:  1,
	}

	if err := s.RefreshList(context.Background(), fetcher); err != nil {
		t.Fatal(err)
	}

	if s.NeedsListRefresh() {
		t.Fatal("should not need refresh after fetch")
	}

	traces := s.Traces()
	if len(traces) != 1 || traces[0].ID != "t1" {
		t.Fatalf("unexpected traces: %+v", traces)
	}
}

func TestTraceStore_SelectAndDetail(t *testing.T) {
	s := NewTraceStore()
	s.SelectTrace("t1")

	if !s.NeedsDetailRefresh() {
		t.Fatal("should need detail refresh after select")
	}

	fetcher := &mockTraceFetcher{
		trace: &views.TraceData{ID: "t1", Name: "test"},
		spans: []views.SpanData{
			{ID: "s1", TraceID: "t1", SpanType: "agent", Name: "root"},
			{ID: "s2", TraceID: "t1", ParentSpanID: "s1", SpanType: "llm_call", Name: "call1"},
		},
	}

	if err := s.RefreshDetail(context.Background(), fetcher); err != nil {
		t.Fatal(err)
	}

	if s.NeedsDetailRefresh() {
		t.Fatal("should not need detail refresh after fetch")
	}

	tree := s.SelectedSpanTree()
	if len(tree) != 1 {
		t.Fatalf("expected 1 root, got %d", len(tree))
	}
	if len(tree[0].Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(tree[0].Children))
	}
}

func TestTraceStore_SelectChangeDuringFetch(t *testing.T) {
	s := NewTraceStore()
	s.SelectTrace("t1")

	fetcher := &mockTraceFetcher{
		trace: &views.TraceData{ID: "t1"},
		spans: []views.SpanData{{ID: "s1", TraceID: "t1", SpanType: "agent"}},
	}

	_ = s.RefreshDetail(context.Background(), fetcher)

	// Now change selection
	s.SelectTrace("t2")

	if !s.NeedsDetailRefresh() {
		t.Fatal("should need detail refresh after new selection")
	}
	if s.SelectedID() != "t2" {
		t.Fatal("selectedID should be t2")
	}
}

func TestTraceStore_Filters(t *testing.T) {
	s := NewTraceStore()
	s.SetFilters(views.TraceFilters{AgentID: "bot", Limit: 20})

	f := s.Filters()
	if f.AgentID != "bot" || f.Limit != 20 {
		t.Fatalf("unexpected filters: %+v", f)
	}

	if !s.NeedsListRefresh() {
		t.Fatal("should need refresh after filter change")
	}
}
