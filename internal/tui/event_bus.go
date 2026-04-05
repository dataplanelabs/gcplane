package tui

import (
	"sync"

	"github.com/rivo/tview"
)

// EventType identifies categories of TUI events.
type EventType string

const (
	EventPlanUpdated EventType = "plan.updated"  // new plan from refresh
	EventTraceEntry  EventType = "trace.entry"   // new trace log entry
	EventStatusMsg   EventType = "status.message" // status bar message
)

// Event carries data through the event bus.
type Event struct {
	Type    EventType
	Payload any
}

// EventBus provides publish/subscribe for TUI events.
// All subscriber callbacks are wrapped in QueueUpdateDraw for thread safety.
type EventBus struct {
	tapp        *tview.Application
	subscribers map[EventType][]func(Event)
	mu          sync.RWMutex
}

// NewEventBus creates an event bus bound to the given tview application.
func NewEventBus(tapp *tview.Application) *EventBus {
	return &EventBus{
		tapp:        tapp,
		subscribers: make(map[EventType][]func(Event)),
	}
}

// Subscribe registers a callback for an event type.
// The callback runs inside QueueUpdateDraw (safe for UI updates).
func (b *EventBus) Subscribe(t EventType, fn func(Event)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[t] = append(b.subscribers[t], fn)
}

// Publish sends an event to all subscribers of that type.
// Safe to call from any goroutine.
func (b *EventBus) Publish(e Event) {
	b.mu.RLock()
	subs := b.subscribers[e.Type]
	b.mu.RUnlock()
	if len(subs) == 0 {
		return
	}
	b.tapp.QueueUpdateDraw(func() {
		for _, fn := range subs {
			fn(e)
		}
	})
}
