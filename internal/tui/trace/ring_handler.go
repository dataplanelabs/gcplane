// Package trace provides slog capture into a ring buffer for TUI display.
package trace

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"
)

// Entry is a captured log event for TUI display.
type Entry struct {
	Time    time.Time
	Level   slog.Level
	Message string
	Attrs   map[string]any
}

// ringCore holds the shared ring buffer state across handler clones.
type ringCore struct {
	entries []Entry
	size    int
	pos     int
	mu      sync.Mutex
	onEntry func(Entry)
}

// RingHandler implements slog.Handler, storing entries in a fixed-size ring buffer.
type RingHandler struct {
	core  *ringCore
	level slog.Level
	attrs []slog.Attr
}

// NewRingHandler creates a handler with the given buffer size and minimum log level.
// onEntry is called on each new entry (may be nil).
func NewRingHandler(size int, level slog.Level, onEntry func(Entry)) *RingHandler {
	return &RingHandler{
		core: &ringCore{
			entries: make([]Entry, size),
			size:    size,
			onEntry: onEntry,
		},
		level: level,
	}
}

// SetOnEntry sets the callback invoked on each new entry.
// Safe to call after construction (e.g., after event bus is wired).
func (h *RingHandler) SetOnEntry(fn func(Entry)) {
	h.core.mu.Lock()
	defer h.core.mu.Unlock()
	h.core.onEntry = fn
}

// Enabled reports whether the handler handles records at the given level.
func (h *RingHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle stores the record in the ring buffer and calls onEntry.
func (h *RingHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := make(map[string]any, len(h.attrs)+r.NumAttrs())
	for _, a := range h.attrs {
		attrs[a.Key] = a.Value.Any()
	}
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	entry := Entry{
		Time:    r.Time,
		Level:   r.Level,
		Message: r.Message,
		Attrs:   attrs,
	}

	c := h.core
	c.mu.Lock()
	c.entries[c.pos%c.size] = entry
	c.pos++
	c.mu.Unlock()

	if c.onEntry != nil {
		c.onEntry(entry)
	}
	return nil
}

// WithAttrs returns a new handler with the given attrs pre-set.
// The child shares the same ring buffer core.
func (h *RingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &RingHandler{
		core:  h.core,
		level: h.level,
		attrs: append(slices.Clone(h.attrs), attrs...),
	}
}

// WithGroup returns the handler unchanged (groups flattened for simplicity).
func (h *RingHandler) WithGroup(_ string) slog.Handler {
	return h
}

// Snapshot returns entries in chronological order.
func (h *RingHandler) Snapshot() []Entry {
	c := h.core
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pos == 0 {
		return nil
	}
	if c.pos <= c.size {
		return slices.Clone(c.entries[:c.pos])
	}
	start := c.pos % c.size
	result := make([]Entry, c.size)
	copy(result, c.entries[start:])
	copy(result[c.size-start:], c.entries[:start])
	return result
}

// Count returns total entries received (may exceed buffer size).
func (h *RingHandler) Count() int {
	c := h.core
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pos
}
