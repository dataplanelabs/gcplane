package views

import (
	"encoding/json"
	"time"
)

// LogEntry is a parsed GoClaw server log for display in the Logs panel.
type LogEntry struct {
	Time    time.Time
	Level   string // "debug", "info", "warn", "error"
	Message string
	Source  string
	Attrs   map[string]string
}

// LiveEvent is a parsed GoClaw push event for display in the Events panel.
type LiveEvent struct {
	Time    time.Time
	Type    string          // event name: "agent", "health", "cron", etc.
	Subtype string          // payload.type if present: "run.started", "tool.call"
	Summary string          // human-readable one-line summary
	Raw     json.RawMessage // full payload for drill-down
}
