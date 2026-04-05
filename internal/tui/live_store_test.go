package tui

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/dataplanelabs/gcplane/internal/provider/goclaw"
)

func TestLiveStore_LogRingBuffer(t *testing.T) {
	s := NewLiveStore(3, 3)

	// Fill buffer
	for i := range 5 {
		s.HandleEvent(goclaw.WSEventFrame{
			Type:    "event",
			Event:   "log",
			Payload: mustJSON(t, map[string]any{"timestamp": int64(1000 + i), "level": "info", "message": msgN(i)}),
		})
	}

	if s.LogCount() != 5 {
		t.Fatalf("want LogCount=5, got %d", s.LogCount())
	}

	logs := s.Logs()
	if len(logs) != 3 {
		t.Fatalf("want 3 logs (ring cap), got %d", len(logs))
	}

	// Chronological: oldest first → msg2, msg3, msg4
	if logs[0].Message != "msg2" {
		t.Errorf("want logs[0]=msg2, got %s", logs[0].Message)
	}
	if logs[2].Message != "msg4" {
		t.Errorf("want logs[2]=msg4, got %s", logs[2].Message)
	}
}

func TestLiveStore_EventRingBuffer(t *testing.T) {
	s := NewLiveStore(3, 2)

	for _, evt := range []string{"run.started", "run.completed", "run.failed"} {
		s.HandleEvent(goclaw.WSEventFrame{
			Type:    "event",
			Event:   "agent",
			Payload: mustJSON(t, map[string]any{"type": evt, "agent": "bot"}),
		})
	}

	if s.EventCount() != 3 {
		t.Fatalf("want EventCount=3, got %d", s.EventCount())
	}

	events := s.Events()
	if len(events) != 2 {
		t.Fatalf("want 2 events (ring cap), got %d", len(events))
	}

	// Chronological: oldest surviving → run.completed, run.failed
	if events[0].Subtype != "run.completed" {
		t.Errorf("want events[0].Subtype=run.completed, got %s", events[0].Subtype)
	}
}

func TestLiveStore_DirtyFlag(t *testing.T) {
	s := NewLiveStore(10, 10)

	if s.IsDirty() {
		t.Fatal("new store should not be dirty")
	}

	s.HandleEvent(goclaw.WSEventFrame{
		Type:    "event",
		Event:   "health",
		Payload: mustJSON(t, map[string]any{}),
	})

	if !s.IsDirty() {
		t.Fatal("store should be dirty after event")
	}

	s.MarkClean()
	if s.IsDirty() {
		t.Fatal("store should be clean after MarkClean")
	}
}

func TestLiveStore_ConcurrentAccess(t *testing.T) {
	s := NewLiveStore(100, 100)
	var wg sync.WaitGroup

	// Writer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 50 {
			s.HandleEvent(goclaw.WSEventFrame{
				Type:    "event",
				Event:   "log",
				Payload: mustJSON(t, map[string]any{"timestamp": int64(i), "level": "info", "message": "msg"}),
			})
		}
	}()

	// Reader goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 50 {
			_ = s.Logs()
			_ = s.Events()
			_ = s.IsDirty()
		}
	}()

	wg.Wait()
}

func TestLiveStore_AgentEventSummary(t *testing.T) {
	s := NewLiveStore(10, 10)
	s.HandleEvent(goclaw.WSEventFrame{
		Type:    "event",
		Event:   "agent",
		Payload: mustJSON(t, map[string]any{"type": "tool.call", "agent": "helper", "tool": "search"}),
	})
	events := s.Events()
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	if events[0].Summary != "Agent helper -> tool.search" {
		t.Errorf("unexpected summary: %s", events[0].Summary)
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func msgN(i int) string {
	return "msg" + string(rune('0'+i))
}
