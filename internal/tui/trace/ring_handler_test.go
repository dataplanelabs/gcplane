package trace

import (
	"log/slog"
	"sync"
	"testing"
)

func TestRingHandlerBasicWrite(t *testing.T) {
	h := NewRingHandler(10, slog.LevelDebug, nil)
	logger := slog.New(h)

	logger.Info("hello", slog.String("key", "val"))

	entries := h.Snapshot()
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Message != "hello" {
		t.Errorf("want message 'hello', got %q", entries[0].Message)
	}
	if entries[0].Attrs["key"] != "val" {
		t.Errorf("want attr key=val, got %v", entries[0].Attrs["key"])
	}
}

func TestRingHandlerWraparound(t *testing.T) {
	h := NewRingHandler(3, slog.LevelDebug, nil)
	logger := slog.New(h)

	for i := range 5 {
		logger.Info("msg", slog.Int("i", i))
	}

	entries := h.Snapshot()
	if len(entries) != 3 {
		t.Fatalf("want 3 entries after wraparound, got %d", len(entries))
	}
	// Should have entries 2, 3, 4 in order
	for idx, want := range []int64{2, 3, 4} {
		got, ok := entries[idx].Attrs["i"].(int64)
		if !ok || got != want {
			t.Errorf("entry[%d]: want i=%d, got %v", idx, want, entries[idx].Attrs["i"])
		}
	}
	if h.Count() != 5 {
		t.Errorf("want count 5, got %d", h.Count())
	}
}

func TestRingHandlerEmptySnapshot(t *testing.T) {
	h := NewRingHandler(10, slog.LevelDebug, nil)
	entries := h.Snapshot()
	if entries != nil {
		t.Errorf("want nil for empty buffer, got %v", entries)
	}
}

func TestRingHandlerLevelFilter(t *testing.T) {
	h := NewRingHandler(10, slog.LevelWarn, nil)
	logger := slog.New(h)

	logger.Debug("skip")
	logger.Info("skip")
	logger.Warn("keep")
	logger.Error("keep")

	entries := h.Snapshot()
	if len(entries) != 2 {
		t.Fatalf("want 2 entries (WARN+ERROR), got %d", len(entries))
	}
}

func TestRingHandlerOnEntryCallback(t *testing.T) {
	var count int
	h := NewRingHandler(10, slog.LevelDebug, func(_ Entry) { count++ })
	logger := slog.New(h)

	logger.Info("one")
	logger.Info("two")

	if count != 2 {
		t.Errorf("want onEntry called 2 times, got %d", count)
	}
}

func TestRingHandlerConcurrentWrites(t *testing.T) {
	h := NewRingHandler(100, slog.LevelDebug, nil)
	logger := slog.New(h)

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				logger.Info("concurrent")
			}
		}()
	}
	wg.Wait()

	if h.Count() != 500 {
		t.Errorf("want 500 total writes, got %d", h.Count())
	}
	entries := h.Snapshot()
	if len(entries) != 100 {
		t.Errorf("want 100 entries in buffer, got %d", len(entries))
	}
}

func TestRingHandlerWithAttrs(t *testing.T) {
	h := NewRingHandler(10, slog.LevelDebug, nil)
	logger := slog.New(h).With(slog.String("component", "engine"))

	logger.Info("test")

	entries := h.Snapshot()
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Attrs["component"] != "engine" {
		t.Errorf("want component=engine, got %v", entries[0].Attrs["component"])
	}
}
