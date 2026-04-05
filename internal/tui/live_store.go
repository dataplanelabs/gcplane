package tui

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/dataplanelabs/gcplane/internal/provider/goclaw"
	"github.com/dataplanelabs/gcplane/internal/tui/views"
)

// LiveStore holds ring-buffered state from GoClaw WS events.
// Thread-safe: written from WS goroutine, read from UI goroutine.
type LiveStore struct {
	mu      sync.RWMutex
	logs    []views.LogEntry
	events  []views.LiveEvent
	logCap  int
	evtCap  int
	logPos  int  // total logs received (indexes ring)
	evtPos  int  // total events received
	dirty   bool // true if new data since last MarkClean
	tailing bool // true if logs.tail is active
}

// NewLiveStore creates a store with the given ring buffer capacities.
func NewLiveStore(logCap, evtCap int) *LiveStore {
	return &LiveStore{
		logs:   make([]views.LogEntry, 0, logCap),
		events: make([]views.LiveEvent, 0, evtCap),
		logCap: logCap,
		evtCap: evtCap,
	}
}

// HandleEvent processes a raw WS event frame from the read goroutine.
func (s *LiveStore) HandleEvent(frame goclaw.WSEventFrame) {
	switch frame.Event {
	case "log":
		var raw struct {
			Timestamp int64             `json:"timestamp"`
			Level     string            `json:"level"`
			Message   string            `json:"message"`
			Source    string            `json:"source"`
			Attrs     map[string]string `json:"attrs"`
		}
		if err := json.Unmarshal(frame.Payload, &raw); err != nil {
			return
		}
		entry := views.LogEntry{
			Time:    time.UnixMilli(raw.Timestamp),
			Level:   raw.Level,
			Message: raw.Message,
			Source:  raw.Source,
			Attrs:   raw.Attrs,
		}
		s.appendLog(entry)

	default:
		// All other events go to the Events ring buffer.
		evt := parseLiveEvent(frame)
		if evt.Summary != "" {
			s.appendEvent(evt)
		}
	}
}

// Logs returns a chronological snapshot of log entries.
func (s *LiveStore) Logs() []views.LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.logSnapshot()
}

// Events returns a chronological snapshot of live events.
func (s *LiveStore) Events() []views.LiveEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.evtSnapshot()
}

// MarkClean resets the dirty flag. Called after UI redraw.
func (s *LiveStore) MarkClean() {
	s.mu.Lock()
	s.dirty = false
	s.mu.Unlock()
}

// IsDirty returns true if new data arrived since last MarkClean.
func (s *LiveStore) IsDirty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dirty
}

// SetTailing records whether logs.tail is active.
func (s *LiveStore) SetTailing(v bool) {
	s.mu.Lock()
	s.tailing = v
	s.mu.Unlock()
}

// IsTailing returns whether logs.tail subscription is active.
func (s *LiveStore) IsTailing() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tailing
}

// LogCount returns total logs received (may exceed ring capacity).
func (s *LiveStore) LogCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.logPos
}

// EventCount returns total events received.
func (s *LiveStore) EventCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.evtPos
}

// appendLog adds a log entry to the ring buffer.
func (s *LiveStore) appendLog(e views.LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.logPos < s.logCap {
		s.logs = append(s.logs, e)
	} else {
		s.logs[s.logPos%s.logCap] = e
	}
	s.logPos++
	s.dirty = true
}

// appendEvent adds a live event to the ring buffer.
func (s *LiveStore) appendEvent(e views.LiveEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.evtPos < s.evtCap {
		s.events = append(s.events, e)
	} else {
		s.events[s.evtPos%s.evtCap] = e
	}
	s.evtPos++
	s.dirty = true
}

// logSnapshot returns a chronological clone of the logs ring buffer.
func (s *LiveStore) logSnapshot() []views.LogEntry {
	if s.logPos == 0 {
		return nil
	}
	if s.logPos <= s.logCap {
		return slices.Clone(s.logs[:s.logPos])
	}
	start := s.logPos % s.logCap
	result := make([]views.LogEntry, s.logCap)
	copy(result, s.logs[start:])
	copy(result[s.logCap-start:], s.logs[:start])
	return result
}

// evtSnapshot returns a chronological clone of the events ring buffer.
func (s *LiveStore) evtSnapshot() []views.LiveEvent {
	if s.evtPos == 0 {
		return nil
	}
	if s.evtPos <= s.evtCap {
		return slices.Clone(s.events[:s.evtPos])
	}
	start := s.evtPos % s.evtCap
	result := make([]views.LiveEvent, s.evtCap)
	copy(result, s.events[start:])
	copy(result[s.evtCap-start:], s.events[:start])
	return result
}

// parseLiveEvent extracts type, subtype, and summary from a WS event frame.
func parseLiveEvent(frame goclaw.WSEventFrame) views.LiveEvent {
	evt := views.LiveEvent{
		Time: time.Now(),
		Type: frame.Event,
		Raw:  frame.Payload,
	}

	var peek struct {
		Type    string `json:"type"`
		Agent   string `json:"agent"`
		Tool    string `json:"tool"`
		Status  string `json:"status"`
		Message string `json:"message"`
		Name    string `json:"name"`
	}
	if frame.Payload != nil {
		_ = json.Unmarshal(frame.Payload, &peek)
	}
	evt.Subtype = peek.Type

	switch frame.Event {
	case "agent":
		agent := peek.Agent
		if agent == "" {
			agent = peek.Name
		}
		switch peek.Type {
		case "run.started":
			evt.Summary = fmt.Sprintf("Agent %s run started", agent)
		case "run.completed":
			evt.Summary = fmt.Sprintf("Agent %s run completed", agent)
		case "run.failed":
			evt.Summary = fmt.Sprintf("Agent %s run FAILED", agent)
		case "run.cancelled":
			evt.Summary = fmt.Sprintf("Agent %s run cancelled", agent)
		case "tool.call":
			evt.Summary = fmt.Sprintf("Agent %s -> tool.%s", agent, peek.Tool)
		case "tool.result":
			evt.Summary = fmt.Sprintf("Agent %s <- tool.%s", agent, peek.Tool)
		case "block.reply":
			evt.Summary = fmt.Sprintf("Agent %s block reply", agent)
		default:
			evt.Summary = fmt.Sprintf("Agent %s: %s", agent, peek.Type)
		}
	case "chat":
		switch peek.Type {
		case "message":
			evt.Summary = "Chat message"
		case "chunk":
			evt.Summary = "Chat chunk"
		case "thinking":
			evt.Summary = "Chat thinking"
		default:
			evt.Summary = "Chat: " + peek.Type
		}
	case "health":
		evt.Summary = "System health update"
	case "cron":
		evt.Summary = fmt.Sprintf("Cron: %s", peek.Status)
		if peek.Name != "" {
			evt.Summary = fmt.Sprintf("Cron %s: %s", peek.Name, peek.Status)
		}
	case "presence":
		evt.Summary = "Presence update"
	case "session.updated":
		evt.Summary = "Session updated"
	case "trace.updated":
		evt.Summary = "Trace updated"
	case "heartbeat":
		evt.Summary = "Heartbeat"
	case "shutdown":
		evt.Summary = "Server shutdown"
	default:
		// Team events and others
		if strings.HasPrefix(frame.Event, "team.") {
			evt.Summary = formatTeamEvent(frame.Event, peek.Type)
		} else {
			evt.Summary = frame.Event
			if peek.Type != "" {
				evt.Summary += ": " + peek.Type
			}
		}
	}
	return evt
}

// formatTeamEvent produces a summary for team-related events.
func formatTeamEvent(event, subtype string) string {
	// e.g. "team.task.created" → "Team task created"
	short := strings.TrimPrefix(event, "team.")
	short = strings.ReplaceAll(short, ".", " ")
	return "Team " + short
}
